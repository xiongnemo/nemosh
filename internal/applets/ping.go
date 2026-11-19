package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/xiongnemo/nemosh/internal/netio"
)

// ping, iputils', which takes the name of Windows' PING.EXE: that one answers -c with `Option -c
// requires administrative privileges`, and still runs as ping.exe. busybox-w32 has no ping. An echo
// is IcmpSendEcho2's or Icmp6SendEcho2's (internal/netio), which any process may send.
//
// The words are iputils' (ping_common.c): `PING HOST (ADDR) 56(84) bytes of data.`, a reply's
// line with its time to three places under a millisecond, two under ten, one under a hundred and
// none above, `From X icmp_seq=N Destination Host Unreachable`, and after a blank line the
// statistics, the loss as %g writes it, rtt min/avg/max/mdev to the microsecond. A HOST that is a
// name is looked up and each reply's address named, but with -n or when the HOST was an address.
// An IPv6 reply has no ttl=: Windows does not say what hop limit it came with.
//
// Each echo is waited for -W seconds, or an interval's, a second at least, where iputils waits for
// any reply until the end. Status 0 when something answered, 1 when nothing did, 2 on an error.

type pingSource struct {
	resolve func(context.Context, string) ([]netip.Addr, error)
	reverse func(context.Context, string) ([]string, error)
	echo    func(to netip.Addr, size, ttl int, timeout time.Duration) (netio.EchoReply, error)
	now     func() time.Time
	sleep   func(context.Context, time.Duration) error
}

type pingSettings struct {
	count, size, ttl         int
	interval, wait, deadline time.Duration
	quiet, numeric, overdue  bool
	family                   int
}

func newPingApplet() Applet {
	return simpleApplet{name: "ping", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		pingers := map[bool]*netio.Pinger{}
		defer func() {
			for _, pinger := range pingers {
				pinger.Close()
			}
		}()
		source := pingSource{resolve: netio.LookupHost, reverse: netio.LookupAddr, now: time.Now, sleep: sleepContext,
			echo: func(to netip.Addr, size, ttl int, timeout time.Duration) (netio.EchoReply, error) {
				pinger, opened := pingers[to.Is6()]
				if !opened {
					var err error
					if pinger, err = netio.NewPinger(to.Is6()); err != nil {
						return netio.EchoReply{}, err
					}
					pingers[to.Is6()] = pinger
				}
				return pinger.Echo(to, size, ttl, timeout)
			}}
		return runPing(ctx, args, source, stdout)
	}}
}

func runPing(ctx context.Context, args []string, source pingSource, stdout io.Writer) error {
	settings, host, err := pingArguments(ctx, args)
	if err != nil {
		return err
	}
	addresses, err := source.resolve(ctx, host)
	if err != nil {
		return ExitStatusMessage(2, fmt.Errorf("%s: Name or service not known", host))
	}
	target, found := netip.Addr{}, false
	for _, address := range addresses {
		if settings.family == 0 || settings.family == 4 && address.Is4() || settings.family == 6 && address.Is6() {
			target, found = address, true
			break
		}
	}
	if !found {
		return ExitStatusMessage(2, fmt.Errorf("%s: Address family for hostname not supported", host))
	}
	_, numericHost := netip.ParseAddr(host)
	name := func(address netip.Addr) string {
		if settings.numeric || numericHost == nil {
			return address.String()
		}
		if names, err := source.reverse(ctx, address.String()); err == nil && len(names) > 0 {
			return strings.TrimSuffix(names[0], ".") + " (" + address.String() + ")"
		}
		return address.String() + " (" + address.String() + ")"
	}
	if target.Is6() {
		fmt.Fprintf(stdout, "PING %s (%s) %d data bytes\n", host, target, settings.size)
	} else {
		fmt.Fprintf(stdout, "PING %s (%s) %d(%d) bytes of data.\n", host, target, settings.size, settings.size+28)
	}
	run := pingRun{settings: settings, source: source, stdout: stdout, name: name}
	run.loop(ctx, target)
	return run.report(host)
}

type pingRun struct {
	settings                     pingSettings
	source                       pingSource
	stdout                       io.Writer
	name                         func(netip.Addr) string
	transmitted, received, fails int
	times                        []time.Duration
	elapsed                      time.Duration
}

// loop sends an echo an interval, until -c's count, -w's deadline, or an interrupt.
func (r *pingRun) loop(ctx context.Context, target netip.Addr) {
	start := r.source.now()
	defer func() { r.elapsed = r.source.now().Sub(start) }()
	wait := r.settings.wait
	if wait == 0 {
		wait = max(r.settings.interval, time.Second)
	}
	for sequence := 1; r.settings.count == 0 || sequence <= r.settings.count; sequence++ {
		if ctx.Err() != nil || r.settings.deadline > 0 && r.source.now().Sub(start) >= r.settings.deadline {
			return
		}
		sent := r.source.now()
		reply, err := r.source.echo(target, r.settings.size, r.settings.ttl, wait)
		r.transmitted++
		switch {
		case err != nil:
			r.fails++
			fmt.Fprintf(r.stdout, "ping: sendmsg: %v\n", err)
		case reply.Status == 0:
			r.received++
			r.times = append(r.times, reply.RTT)
			if !r.settings.quiet {
				ttl := ""
				if reply.TTL >= 0 {
					ttl = fmt.Sprintf(" ttl=%d", reply.TTL)
				}
				fmt.Fprintf(r.stdout, "%d bytes from %s: icmp_seq=%d%s time=%s ms\n", r.settings.size+8, r.name(reply.From), sequence, ttl, pingTime(reply.RTT))
			}
		case reply.Status == netio.EchoTimedOut:
			if r.settings.overdue && !r.settings.quiet {
				fmt.Fprintf(r.stdout, "no answer yet for icmp_seq=%d\n", sequence)
			}
		default:
			r.fails++
			if !r.settings.quiet {
				fmt.Fprintf(r.stdout, "From %s icmp_seq=%d %s\n", r.name(reply.From), sequence, pingStatus(reply.Status))
			}
		}
		if r.settings.count > 0 && sequence == r.settings.count {
			return
		}
		if pause := r.settings.interval - r.source.now().Sub(sent); pause > 0 {
			if r.source.sleep(ctx, pause) != nil {
				return
			}
		}
	}
}

// report writes the statistics and answers ping's status.
func (r *pingRun) report(host string) error {
	fmt.Fprintf(r.stdout, "\n--- %s ping statistics ---\n%d packets transmitted, %d received", host, r.transmitted, r.received)
	if r.fails > 0 {
		fmt.Fprintf(r.stdout, ", +%d errors", r.fails)
	}
	loss := 0.0
	if r.transmitted > 0 {
		loss = float64(r.transmitted-r.received) * 100 / float64(r.transmitted)
	}
	fmt.Fprintf(r.stdout, ", %s%% packet loss, time %dms\n", strconv.FormatFloat(loss, 'g', 6, 64), r.elapsed.Milliseconds())
	if r.received > 0 {
		least, most, sum, squares := r.times[0], r.times[0], 0.0, 0.0
		for _, rtt := range r.times {
			least, most = min(least, rtt), max(most, rtt)
			micro := float64(rtt.Microseconds())
			sum, squares = sum+micro, squares+micro*micro
		}
		average := sum / float64(len(r.times))
		deviation := math.Sqrt(max(0, squares/float64(len(r.times))-average*average))
		fmt.Fprintf(r.stdout, "rtt min/avg/max/mdev = %s/%s/%s/%s ms\n", pingMillis(least.Microseconds()), pingMillis(int64(average)),
			pingMillis(most.Microseconds()), pingMillis(int64(deviation)))
	}
	if r.received == 0 || r.settings.count > 0 && r.settings.deadline > 0 && r.received < r.settings.count {
		return ExitStatus(1)
	}
	return nil
}

// pingTime is a reply's time as iputils writes it: three places under a millisecond, two under
// ten, one under a hundred, none above.
func pingTime(rtt time.Duration) string {
	micro := rtt.Microseconds()
	switch {
	case micro >= 100000-50:
		return strconv.FormatInt((micro+500)/1000, 10)
	case micro >= 10000-5:
		return fmt.Sprintf("%d.%01d", (micro+50)/1000, (micro+50)%1000/100)
	case micro >= 1000:
		return fmt.Sprintf("%d.%02d", (micro+5)/1000, (micro+5)%1000/10)
	}
	return pingMillis(micro)
}

func pingMillis(micro int64) string { return fmt.Sprintf("%d.%03d", micro/1000, micro%1000) }

// pingStatus is an IP_STATUS in the words iputils gives the ICMP message it stands for.
func pingStatus(status uint32) string {
	switch status {
	case 11002:
		return "Destination Net Unreachable"
	case 11003:
		return "Destination Host Unreachable"
	case 11004:
		return "Destination Protocol Unreachable"
	case 11005:
		return "Destination Port Unreachable"
	case 11013:
		return "Time to live exceeded"
	}
	return fmt.Sprintf("failed, IP status %d", status)
}

// sleepContext waits, or answers the context's end.
func sleepContext(ctx context.Context, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

var errPingWindowsHabit = errors.New("this is Linux's ping, `ping -c COUNT HOST`; Windows' runs as ping.exe")
