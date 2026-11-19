package applets

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/rivo/uniseg"
	"golang.org/x/sys/windows"

	"github.com/xiongnemo/nemosh/internal/netio"
	"github.com/xiongnemo/nemosh/internal/proc"
)

// ss, iproute2's, from Windows' socket tables (GetExtendedTcpTable and GetExtendedUdpTable), which
// say of every socket the process that owns it without an administrator. busybox-w32 has no ss,
// and Windows' netstat answers -tulpn with its usage.
//
//   - -t and -u, both when neither is given, and the Netid column then, UDP before TCP, as ss
//     dumps them; -4 and -6;
//   - by default the connected sockets but TIME-WAIT and SYN-RECV, as ss's SS_CONN has it; -l the
//     listening ones, which for UDP are all of them, UNCONN; -a every one;
//   - a port by its name from Windows' services file, as ss names it from /etc/services, unless
//     -n; 0 is *; an IPv6 address in brackets;
//   - -p `users:(("NAME",pid=N))`, the image name, there being no fd to name;
//   - -H no header.
//
// Recv-Q and Send-Q are -: Windows says them of no socket without an administrator's tracing.
// The columns are as wide as what is in them, where ss widens them to the screen. -s, which
// counts what Linux's /proc keeps, and a filter expression are refused with the reason.

type ssSource struct {
	sockets   func() ([]netio.Socket, error)
	processes func() (map[int]string, error)
	services  func(port uint16, protocol string) string
}

const ssLongOptions = "tcp=t udp=u listening=l all=a numeric=n processes=p ipv4=4 ipv6=6 no-header=H summary=s"

func newSsApplet() Applet {
	return simpleApplet{name: "ss", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		return runSs(ctx, args, ssSource{sockets: netio.Sockets, processes: ssProcesses, services: ssServices}, stdout)
	}}
}

func runSs(ctx context.Context, args []string, source ssSource, stdout io.Writer) error {
	options, operands, err := parseAppletLongOptions(ctx, args, ssLongOptions, "tulanp46Hs", "")
	switch {
	case err != nil:
		return err
	case options.has('s'):
		return errors.New("-s counts what Linux's /proc keeps, which Windows has not; `ss -ta | wc -l` counts the sockets")
	case len(operands) > 0:
		return fmt.Errorf("%s: ss's filter expressions are not here; the lines are there to grep", operands[0])
	}
	tcp, udp := options.has('t'), options.has('u')
	if !tcp && !udp {
		tcp, udp = true, true
	}
	sockets, err := source.sockets()
	if err != nil {
		return err
	}
	sockets = slices.DeleteFunc(sockets, func(socket netio.Socket) bool {
		four := socket.Local.Addr().Is4()
		return socket.Protocol == "tcp" && !tcp || socket.Protocol == "udp" && !udp ||
			options.has('4') && !four || options.has('6') && four || !ssShown(socket, options.has('l'), options.has('a'))
	})
	slices.SortStableFunc(sockets, func(a, b netio.Socket) int { return cmp.Compare(b.Protocol, a.Protocol) })
	var names map[int]string
	if options.has('p') {
		if names, err = source.processes(); err != nil {
			return err
		}
	}
	rows := [][]string{{"Netid", "State", "Recv-Q", "Send-Q", "Local Address:", "Port", "Peer Address:", "Port", "Process"}}
	for _, socket := range sockets {
		local, localPort := ssEnd(socket.Local, socket.Protocol, options.has('n'), source.services)
		peer, peerPort := ssEnd(socket.Peer, socket.Protocol, options.has('n'), source.services)
		process := ""
		if name, known := names[socket.PID]; options.has('p') && (known || socket.PID != 0) {
			process = fmt.Sprintf(`users:(("%s",pid=%d))`, name, socket.PID)
		}
		rows = append(rows, []string{socket.Protocol, ssState(socket), "-", "-", local, localPort, peer, peerPort, process})
	}
	if !tcp || !udp {
		for index := range rows {
			rows[index] = rows[index][1:]
		}
	}
	if options.has('H') {
		rows = rows[1:]
	}
	return writeSsTable(stdout, rows, !tcp || !udp)
}

// ssShown is whether the socket is listed: every one with -a, the listening ones with -l -- for UDP
// that is all of them -- and otherwise the connected ones but TIME-WAIT and SYN-RECV.
func ssShown(socket netio.Socket, listening, all bool) bool {
	listens := socket.Protocol == "udp" || socket.State == netio.TCPListen || socket.State == 1
	switch {
	case all:
		return true
	case listening:
		return listens
	}
	return !listens && socket.State != netio.TCPTimeWait && socket.State != 4
}

func ssState(socket netio.Socket) string {
	if socket.Protocol == "udp" {
		return "UNCONN"
	}
	states := [...]string{"UNKNOWN", "UNCONN", "LISTEN", "SYN-SENT", "SYN-RECV", "ESTAB", "FIN-WAIT-1", "FIN-WAIT-2", "CLOSE-WAIT", "CLOSING", "LAST-ACK", "TIME-WAIT"}
	if int(socket.State) < len(states) {
		return states[socket.State]
	}
	return "UNKNOWN"
}

// ssEnd is a socket's end as ss writes it: the address and its colon, and the port or its name.
func ssEnd(end netip.AddrPort, protocol string, numeric bool, services func(uint16, string) string) (string, string) {
	address := end.Addr().String()
	if end.Addr().Is6() {
		address = "[" + address + "]"
	}
	port := strconv.Itoa(int(end.Port()))
	switch {
	case end.Port() == 0:
		port = "*"
	case !numeric:
		if name := services(end.Port(), protocol); name != "" {
			port = name
		}
	}
	return address + ":", port
}

// writeSsTable writes the rows in columns as wide as what is in them: left but for the addresses,
// which are right, so their colons line up before the ports, as ss's columns[] has them.
func writeSsTable(stdout io.Writer, rows [][]string, withoutNetid bool) error {
	right := []bool{false, false, false, false, true, false, true, false, false}
	separators := []string{"", " ", " ", " ", " ", "", " ", "", " "}
	if withoutNetid {
		right, separators = right[1:], append([]string{""}, separators[2:]...)
	}
	widths := make([]int, len(right))
	for _, row := range rows {
		for column, cell := range row {
			widths[column] = max(widths[column], uniseg.StringWidth(cell))
		}
	}
	out := bufio.NewWriter(stdout)
	for _, row := range rows {
		var line strings.Builder
		for column, cell := range row {
			padding := strings.Repeat(" ", widths[column]-uniseg.StringWidth(cell))
			line.WriteString(separators[column])
			if right[column] {
				line.WriteString(padding + cell)
			} else {
				line.WriteString(cell + padding)
			}
		}
		out.WriteString(strings.TrimRight(line.String(), " ") + "\n")
	}
	return out.Flush()
}

// ssProcesses names every process by its image, from the system's table.
func ssProcesses() (map[int]string, error) {
	snapshot, err := proc.NewSampler().Sample(false)
	if err != nil {
		return nil, err
	}
	names := make(map[int]string, len(snapshot.Processes))
	for _, process := range snapshot.Processes {
		names[process.PID] = process.Name
	}
	return names, nil
}

var ssServiceTable struct {
	once  sync.Once
	names map[string]string
}

// ssServices is a port's name in Windows' services file, %SystemRoot%\System32\drivers\etc\services,
// which has /etc/services' form and is read once.
func ssServices(port uint16, protocol string) string {
	ssServiceTable.once.Do(func() {
		ssServiceTable.names = map[string]string{}
		directory, err := windows.GetSystemDirectory()
		if err != nil {
			return
		}
		file, err := os.Open(filepath.Join(directory, "drivers", "etc", "services"))
		if err != nil {
			return
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line, _, _ := strings.Cut(scanner.Text(), "#")
			if fields := strings.Fields(line); len(fields) >= 2 {
				if _, taken := ssServiceTable.names[fields[1]]; !taken {
					ssServiceTable.names[fields[1]] = fields[0]
				}
			}
		}
	})
	return ssServiceTable.names[strconv.Itoa(int(port))+"/"+protocol]
}
