package netio

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Socket is one TCP or UDP socket as GetExtendedTcpTable and GetExtendedUdpTable describe it, with
// the process that owns it: its protocol, "tcp" or "udp", its two ends -- a UDP socket's peer is
// unspecified -- its state, MIB_TCP_STATE's for TCP and 0 for UDP, and the owner's pid.
type Socket struct {
	Protocol    string
	Local, Peer netip.AddrPort
	State       uint32
	PID         int
}

// The MIB_TCP_STATE values ss names apart.
const (
	TCPListen      = 2
	TCPEstablished = 5
	TCPTimeWait    = 11
)

var socketTables struct {
	once     sync.Once
	tcp, udp *windows.LazyProc
}

// Sockets is every TCP and UDP socket, IPv4's and then IPv6's, each table in Windows' order.
func Sockets() ([]Socket, error) {
	socketTables.once.Do(func() {
		iphlpapi := windows.NewLazySystemDLL("iphlpapi.dll")
		socketTables.tcp, socketTables.udp = iphlpapi.NewProc("GetExtendedTcpTable"), iphlpapi.NewProc("GetExtendedUdpTable")
	})
	var sockets []Socket
	for _, table := range []struct {
		proc     *windows.LazyProc
		protocol string
		family   uint32
		class    uintptr // TCP_TABLE_OWNER_PID_ALL, UDP_TABLE_OWNER_PID
		row      int
	}{
		{socketTables.tcp, "tcp", windows.AF_INET, 5, 24},
		{socketTables.tcp, "tcp", windows.AF_INET6, 5, 56},
		{socketTables.udp, "udp", windows.AF_INET, 1, 12},
		{socketTables.udp, "udp", windows.AF_INET6, 1, 28},
	} {
		buffer, err := readSocketTable(table.proc, table.family, table.class)
		if err != nil {
			return nil, err
		}
		count := int(binary.LittleEndian.Uint32(buffer))
		for index := range count {
			row := buffer[4+index*table.row : 4+(index+1)*table.row]
			sockets = append(sockets, socketOf(table.protocol, table.family, row))
		}
	}
	return sockets, nil
}

// readSocketTable is one table, read again while it grows between the asking and the reading.
func readSocketTable(proc *windows.LazyProc, family uint32, class uintptr) ([]byte, error) {
	size := uint32(16 << 10)
	for {
		buffer := make([]byte, size)
		const sorted = 1
		result, _, _ := proc.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)), sorted, uintptr(family), class, 0)
		switch err := windows.Errno(result); {
		case result == 0:
			return buffer, nil
		case errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER):
			continue
		default:
			return nil, err
		}
	}
}

// socketOf reads one row: MIB_TCPROW_OWNER_PID, MIB_TCP6ROW_OWNER_PID, MIB_UDPROW_OWNER_PID or
// MIB_UDP6ROW_OWNER_PID, every field a DWORD or bytes, so the layout is the same on every
// architecture. A port is in network order in the low 16 bits of its DWORD.
func socketOf(protocol string, family uint32, row []byte) Socket {
	word := func(at int) uint32 { return binary.LittleEndian.Uint32(row[at:]) }
	port := func(at int) uint16 { return binary.BigEndian.Uint16(row[at:]) }
	four := func(at int) netip.Addr { return netip.AddrFrom4([4]byte(row[at : at+4])) }
	six := func(at int) netip.Addr { return netip.AddrFrom16([16]byte(row[at : at+16])) }
	socket := Socket{Protocol: protocol}
	switch {
	case protocol == "tcp" && family == windows.AF_INET:
		socket.State, socket.PID = word(0), int(word(20))
		socket.Local, socket.Peer = netip.AddrPortFrom(four(4), port(8)), netip.AddrPortFrom(four(12), port(16))
	case protocol == "tcp":
		socket.Local, socket.Peer = netip.AddrPortFrom(six(0), port(20)), netip.AddrPortFrom(six(24), port(44))
		socket.State, socket.PID = word(48), int(word(52))
	case family == windows.AF_INET:
		socket.Local, socket.Peer, socket.PID = netip.AddrPortFrom(four(0), port(4)), netip.AddrPortFrom(netip.IPv4Unspecified(), 0), int(word(8))
	default:
		socket.Local, socket.Peer, socket.PID = netip.AddrPortFrom(six(0), port(20)), netip.AddrPortFrom(netip.IPv6Unspecified(), 0), int(word(24))
	}
	return socket
}
