package applets

import (
	"bytes"
	"context"
	"net/netip"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/netio"
)

// ss is iproute2's, from Windows' socket tables: -t, -u, -l, -a, -n, -p, -4, -6 and -H; the Netid
// column when more than one kind is asked for, UDP before TCP; listening sockets only with -l,
// every one with -a, and by default the connected ones but TIME-WAIT, as ss's SS_CONN has it; a
// port by its name from the services file unless -n; and with -p `users:(("NAME",pid=N))`, there
// being no fd to name. Recv-Q and Send-Q are -: Windows says them of no socket without an
// administrator's tracing. There was no ss.
func TestSs_listsAsIproute2Does(t *testing.T) {
	address := func(text string) netip.AddrPort { return netip.MustParseAddrPort(text) }
	source := ssSource{
		sockets: func() ([]netio.Socket, error) {
			return []netio.Socket{
				{Protocol: "tcp", Local: address("0.0.0.0:22"), Peer: address("0.0.0.0:0"), State: netio.TCPListen, PID: 1200},
				{Protocol: "tcp", Local: address("192.168.1.5:50000"), Peer: address("140.82.112.4:443"), State: netio.TCPEstablished, PID: 3000},
				{Protocol: "tcp", Local: address("192.168.1.5:50001"), Peer: address("140.82.112.4:443"), State: netio.TCPTimeWait},
				{Protocol: "tcp", Local: address("[::]:80"), Peer: address("[::]:0"), State: netio.TCPListen, PID: 4},
				{Protocol: "udp", Local: address("0.0.0.0:5353"), Peer: address("0.0.0.0:0"), PID: 2000},
				{Protocol: "udp", Local: address("[::]:5353"), Peer: address("[::]:0"), PID: 2000},
			}, nil
		},
		processes: func() (map[int]string, error) {
			return map[int]string{1200: "sshd.exe", 3000: "chrome.exe", 4: "System", 2000: "svchost.exe"}, nil
		},
		services: func(port uint16, protocol string) string {
			return map[uint16]string{22: "ssh", 80: "http", 443: "https"}[port]
		},
	}
	run := func(args ...string) []string {
		t.Helper()
		var stdout bytes.Buffer
		if err := runSs(context.Background(), args, source, &stdout); err != nil {
			t.Fatalf("ss %q: %v", args, err)
		}
		return strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	}
	fields := func(lines []string) [][]string {
		var out [][]string
		for _, line := range lines {
			out = append(out, strings.Fields(line))
		}
		return out
	}
	for _, test := range []struct {
		args []string
		want [][]string
	}{
		{[]string{"-tln"}, [][]string{
			{"State", "Recv-Q", "Send-Q", "Local", "Address:Port", "Peer", "Address:Port", "Process"},
			{"LISTEN", "-", "-", "0.0.0.0:22", "0.0.0.0:*"},
			{"LISTEN", "-", "-", "[::]:80", "[::]:*"}}},
		{[]string{"-tl"}, [][]string{
			{"State", "Recv-Q", "Send-Q", "Local", "Address:Port", "Peer", "Address:Port", "Process"},
			{"LISTEN", "-", "-", "0.0.0.0:ssh", "0.0.0.0:*"},
			{"LISTEN", "-", "-", "[::]:http", "[::]:*"}}},
		{[]string{"-tulpn"}, [][]string{
			{"Netid", "State", "Recv-Q", "Send-Q", "Local", "Address:Port", "Peer", "Address:Port", "Process"},
			{"udp", "UNCONN", "-", "-", "0.0.0.0:5353", "0.0.0.0:*", `users:(("svchost.exe",pid=2000))`},
			{"udp", "UNCONN", "-", "-", "[::]:5353", "[::]:*", `users:(("svchost.exe",pid=2000))`},
			{"tcp", "LISTEN", "-", "-", "0.0.0.0:22", "0.0.0.0:*", `users:(("sshd.exe",pid=1200))`},
			{"tcp", "LISTEN", "-", "-", "[::]:80", "[::]:*", `users:(("System",pid=4))`}}},
		{[]string{"-tn"}, [][]string{
			{"State", "Recv-Q", "Send-Q", "Local", "Address:Port", "Peer", "Address:Port", "Process"},
			{"ESTAB", "-", "-", "192.168.1.5:50000", "140.82.112.4:443"}}},
		{[]string{"-tanH4"}, [][]string{
			{"LISTEN", "-", "-", "0.0.0.0:22", "0.0.0.0:*"},
			{"ESTAB", "-", "-", "192.168.1.5:50000", "140.82.112.4:443"},
			{"TIME-WAIT", "-", "-", "192.168.1.5:50001", "140.82.112.4:443"}}},
		{[]string{"-uln6"}, [][]string{
			{"State", "Recv-Q", "Send-Q", "Local", "Address:Port", "Peer", "Address:Port", "Process"},
			{"UNCONN", "-", "-", "[::]:5353", "[::]:*"}}},
	} {
		lines := run(test.args...)
		got := fields(lines)
		if len(got) != len(test.want) {
			t.Errorf("ss %q =\n%s\nwant %d lines", test.args, strings.Join(lines, "\n"), len(test.want))
			continue
		}
		for index := range got {
			if strings.Join(got[index], " ") != strings.Join(test.want[index], " ") {
				t.Errorf("ss %q line %d = %q, want %q", test.args, index, got[index], test.want[index])
			}
		}
		// The columns line up: the colon before each local port is in one column.
		column := -1
		for _, line := range lines {
			at := ssLocalColon(line, test.args)
			if column >= 0 && at != column {
				t.Errorf("ss %q: the local column is not aligned:\n%s", test.args, strings.Join(lines, "\n"))
				break
			}
			column = at
		}
	}
	var stdout bytes.Buffer
	if err := runSs(context.Background(), []string{"-s"}, source, &stdout); err == nil || !strings.Contains(err.Error(), "/proc") {
		t.Errorf("ss -s = %v; want it refused with the reason", err)
	}
}

// ssLocalColon is where the colon before the local port is in a line, or in the header the colon
// of `Local Address:Port`.
func ssLocalColon(line string, args []string) int {
	if at := strings.Index(line, "Local Address:"); at >= 0 {
		return at + len("Local Address")
	}
	field := 3
	if strings.HasPrefix(line, "udp") || strings.HasPrefix(line, "tcp") {
		field = 4
	}
	start := 0
	for index := 0; index < field; index++ {
		start += len(strings.Fields(line[start:])[0])
		start += len(line[start:]) - len(strings.TrimLeft(line[start:], " "))
	}
	local := strings.Fields(line[start:])[0]
	return start + strings.LastIndex(local, ":")
}

// On this machine ss lists what Windows listens on, which is never nothing: RPC's endpoint
// mapper listens on 135 on every Windows there is.
func TestSs_listsThisMachine(t *testing.T) {
	stdout, stderr, status := runApplet(t, "ss", []string{"-tlnp"}, "")
	if status != 0 || !strings.Contains(stdout, "LISTEN") || !strings.Contains(stdout, ":135 ") {
		t.Errorf("ss -tlnp = %q, %s, status %d", stdout, stderr, status)
	}
	t.Logf("ss -tlnp:\n%s", stdout)
}
