package applets

import (
	"bytes"
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/netio"
)

// ping is iputils', by IcmpSendEcho2, which needs no administrator: the header with 56(84) bytes,
// a line a reply in iputils' words with the time to as many places as iputils gives it, `From X
// icmp_seq=N Destination Host Unreachable`, and the statistics after a blank line, the loss as
// %g has it and rtt min/avg/max/mdev to the microsecond. A name is looked up and each reply's
// address named, but with -n or when the HOST was an address. Status 1 when nothing answered.
// It ran Windows' PING.EXE, which answers -c with `Option -c requires administrative privileges`.
func TestPing_asIputilsDoes(t *testing.T) {
	type answer struct {
		rtt    time.Duration
		status uint32
		from   string
		ttl    int
	}
	run := func(answers []answer, args ...string) (string, error) {
		clock := time.Date(2026, 11, 20, 7, 0, 0, 0, time.UTC)
		next := 0
		source := pingSource{
			resolve: func(_ context.Context, host string) ([]netip.Addr, error) {
				switch host {
				case "example.com":
					return []netip.Addr{netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr("2606:2800:220:1:248:1893:25c8:1946")}, nil
				case "nosuch.invalid":
					return nil, errors.New("no such host")
				}
				return []netip.Addr{netip.MustParseAddr(host)}, nil
			},
			reverse: func(_ context.Context, address string) ([]string, error) {
				if address == "93.184.216.34" {
					return []string{"example.org."}, nil
				}
				return nil, errors.New("no name")
			},
			echo: func(to netip.Addr, size, ttl int, timeout time.Duration) (netio.EchoReply, error) {
				reply := answers[next%len(answers)]
				next++
				if reply.status == netio.EchoTimedOut {
					clock = clock.Add(timeout)
					return netio.EchoReply{Status: reply.status, TTL: -1}, nil
				}
				clock = clock.Add(reply.rtt)
				from := to
				if reply.from != "" {
					from = netip.MustParseAddr(reply.from)
				}
				return netio.EchoReply{From: from, Status: reply.status, TTL: reply.ttl, RTT: reply.rtt}, nil
			},
			now:   func() time.Time { return clock },
			sleep: func(_ context.Context, wait time.Duration) error { clock = clock.Add(wait); return nil },
		}
		var stdout bytes.Buffer
		err := runPing(context.Background(), args, source, &stdout)
		return stdout.String(), err
	}
	ms := func(milliseconds float64) time.Duration {
		return time.Duration(milliseconds * float64(time.Millisecond))
	}
	for _, test := range []struct {
		answers []answer
		args    []string
		want    string
		status  int
	}{
		{[]answer{{rtt: ms(11.6), ttl: 56}, {rtt: ms(11.5), ttl: 56}}, []string{"-c", "2", "example.com"},
			"PING example.com (93.184.216.34) 56(84) bytes of data.\n" +
				"64 bytes from example.org (93.184.216.34): icmp_seq=1 ttl=56 time=11.6 ms\n" +
				"64 bytes from example.org (93.184.216.34): icmp_seq=2 ttl=56 time=11.5 ms\n" +
				"\n--- example.com ping statistics ---\n" +
				"2 packets transmitted, 2 received, 0% packet loss, time 1011ms\n" +
				"rtt min/avg/max/mdev = 11.500/11.550/11.600/0.050 ms\n", 0},
		{[]answer{{rtt: 40 * time.Microsecond, ttl: 128}}, []string{"-c", "1", "127.0.0.1"},
			"PING 127.0.0.1 (127.0.0.1) 56(84) bytes of data.\n" +
				"64 bytes from 127.0.0.1: icmp_seq=1 ttl=128 time=0.040 ms\n" +
				"\n--- 127.0.0.1 ping statistics ---\n" +
				"1 packets transmitted, 1 received, 0% packet loss, time 0ms\n" +
				"rtt min/avg/max/mdev = 0.040/0.040/0.040/0.000 ms\n", 0},
		{[]answer{{rtt: ms(1.234), ttl: 64}, {status: netio.EchoTimedOut}, {rtt: ms(123.4), ttl: 64}}, []string{"-n", "-c", "3", "-W", "1", "example.com"},
			"PING example.com (93.184.216.34) 56(84) bytes of data.\n" +
				"64 bytes from 93.184.216.34: icmp_seq=1 ttl=64 time=1.23 ms\n" +
				"64 bytes from 93.184.216.34: icmp_seq=3 ttl=64 time=123 ms\n" +
				"\n--- example.com ping statistics ---\n" +
				"3 packets transmitted, 2 received, 33.3333% packet loss, time 2123ms\n" +
				"rtt min/avg/max/mdev = 1.234/62.317/123.400/61.083 ms\n", 0},
		{[]answer{{status: 11003, from: "192.168.1.1"}}, []string{"-c", "1", "10.9.9.9"},
			"PING 10.9.9.9 (10.9.9.9) 56(84) bytes of data.\n" +
				"From 192.168.1.1 icmp_seq=1 Destination Host Unreachable\n" +
				"\n--- 10.9.9.9 ping statistics ---\n" +
				"1 packets transmitted, 0 received, +1 errors, 100% packet loss, time 0ms\n", 1},
		{[]answer{{rtt: ms(2), ttl: 64}}, []string{"-q", "-c", "2", "-s", "100", "127.0.0.1"},
			"PING 127.0.0.1 (127.0.0.1) 100(128) bytes of data.\n" +
				"\n--- 127.0.0.1 ping statistics ---\n" +
				"2 packets transmitted, 2 received, 0% packet loss, time 1002ms\n" +
				"rtt min/avg/max/mdev = 2.000/2.000/2.000/0.000 ms\n", 0},
		{[]answer{{rtt: ms(0.5), ttl: -1}}, []string{"-6", "-c", "1", "-n", "example.com"},
			"PING example.com (2606:2800:220:1:248:1893:25c8:1946) 56 data bytes\n" +
				"64 bytes from 2606:2800:220:1:248:1893:25c8:1946: icmp_seq=1 time=0.500 ms\n" +
				"\n--- example.com ping statistics ---\n" +
				"1 packets transmitted, 1 received, 0% packet loss, time 0ms\n" +
				"rtt min/avg/max/mdev = 0.500/0.500/0.500/0.000 ms\n", 0},
	} {
		got, err := run(test.answers, test.args...)
		status := 0
		if err != nil {
			status, _ = StatusCode(err)
		}
		if got != test.want || status != test.status {
			t.Errorf("ping %q = %v, status %d\n%s\nwant status %d\n%s", test.args, err, status, got, test.status, test.want)
		}
	}
	for _, test := range []struct {
		args []string
		says string
	}{
		{[]string{"nosuch.invalid"}, "nosuch.invalid: Name or service not known"},
		{[]string{}, "Destination address required"},
		{[]string{"-n", "4", "example.com"}, "ping.exe"},
		{[]string{"/t", "example.com"}, "ping.exe"},
		{[]string{"-c", "0", "example.com"}, "invalid argument"},
	} {
		if _, err := run([]answer{{rtt: ms(1)}}, test.args...); err == nil || !strings.Contains(err.Error(), test.says) {
			t.Errorf("ping %q = %v; want %q", test.args, err, test.says)
		}
	}
}

// On this machine ping reaches the loopback with no administrator, IPv4 and IPv6.
func TestPing_reachesThisMachine(t *testing.T) {
	for _, target := range []string{"127.0.0.1", "::1"} {
		stdout, stderr, status := runApplet(t, "ping", []string{"-c", "1", target}, "")
		if status != 0 || !strings.Contains(stdout, "64 bytes from "+target+": icmp_seq=1 ") || !strings.Contains(stdout, "1 received") {
			t.Errorf("ping -c 1 %s = %q, %s, status %d", target, stdout, stderr, status)
		}
	}
}
