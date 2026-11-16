package netio

import (
	"errors"
	"net/netip"
	"strconv"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Winsock, started the first time a socket is wanted, so a run that opens none pays nothing:
// Go's net package called WSAStartup in its init, a millisecond and more of every start.

var winsock struct {
	once sync.Once
	err  error
}

func winsockStarted() error {
	winsock.once.Do(func() {
		var data windows.WSAData
		winsock.err = windows.WSAStartup(uint32(0x0202), &data)
	})
	return winsock.err
}

// newSocket is an overlapped TCP socket a child process does not inherit.
func newSocket(family int32) (windows.Handle, error) {
	if err := winsockStarted(); err != nil {
		return windows.InvalidHandle, err
	}
	const noHandleInherit = 0x80 // WSA_FLAG_NO_HANDLE_INHERIT
	return windows.WSASocket(family, windows.SOCK_STREAM, windows.IPPROTO_TCP, nil, 0, windows.WSA_FLAG_OVERLAPPED|noHandleInherit)
}

// resolve is the addresses host's port names, by GetAddrInfoW: an address as it is, a name as the
// system's resolver answers it.
func resolve(host, port string) ([]windows.Sockaddr, error) {
	if err := winsockStarted(); err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(host)
	if err != nil {
		return nil, err
	}
	service, err := windows.UTF16PtrFromString(port)
	if err != nil {
		return nil, err
	}
	hints := windows.AddrinfoW{Family: windows.AF_UNSPEC, Socktype: windows.SOCK_STREAM, Protocol: windows.IPPROTO_TCP}
	var found *windows.AddrinfoW
	if err := windows.GetAddrInfoW(name, service, &hints, &found); err != nil {
		return nil, &lookupError{host: host, err: err}
	}
	defer windows.FreeAddrInfoW(found)
	var addresses []windows.Sockaddr
	for entry := found; entry != nil; entry = entry.Next {
		if address, err := (*windows.RawSockaddrAny)(*(*unsafe.Pointer)(unsafe.Pointer(&entry.Addr))).Sockaddr(); err == nil {
			addresses = append(addresses, address)
		}
	}
	if len(addresses) == 0 {
		return nil, &lookupError{host: host, err: errors.New("no address")}
	}
	return addresses, nil
}

type lookupError struct {
	host string
	err  error
}

func (e *lookupError) Error() string { return "lookup " + e.host + ": " + e.err.Error() }
func (e *lookupError) Unwrap() error { return e.err }

// familyOf is the address family a socket for address has to be.
func familyOf(address windows.Sockaddr) int32 {
	if _, ok := address.(*windows.SockaddrInet6); ok {
		return windows.AF_INET6
	}
	return windows.AF_INET
}

// addressText is an address as host:port, an IPv6 one in brackets, as net prints it.
func addressText(address windows.Sockaddr) string {
	switch address := address.(type) {
	case *windows.SockaddrInet4:
		return netip.AddrPortFrom(netip.AddrFrom4(address.Addr), uint16(address.Port)).String()
	case *windows.SockaddrInet6:
		ip := netip.AddrFrom16(address.Addr)
		if address.ZoneId != 0 {
			ip = ip.WithZone(strconv.Itoa(int(address.ZoneId)))
		}
		return netip.AddrPortFrom(ip, uint16(address.Port)).String()
	}
	return ""
}

// anyAddress is the unspecified address of a family, port 0, which ConnectEx needs a socket
// bound to before it connects.
func anyAddress(family int32) windows.Sockaddr {
	if family == windows.AF_INET6 {
		return &windows.SockaddrInet6{}
	}
	return &windows.SockaddrInet4{}
}
