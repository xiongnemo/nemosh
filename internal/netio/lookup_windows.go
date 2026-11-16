package netio

import (
	"context"
	"net/netip"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// LookupAddr is the name an address has, by GetNameInfoW with NI_NAMEREQD, which asks the
// system's resolver as gethostbyaddr does in busybox-w32's ipcalc. In a goroutine of its own, as a
// name is resolved for Dial, so a cancelled context returns at once.
func LookupAddr(ctx context.Context, address string) ([]string, error) {
	ip, err := netip.ParseAddr(address)
	if err != nil {
		return nil, err
	}
	type answer struct {
		name string
		err  error
	}
	answered := make(chan answer, 1)
	go func() {
		name, err := nameOf(ip)
		answered <- answer{name, err}
	}()
	select {
	case found := <-answered:
		if found.err != nil {
			return nil, found.err
		}
		return []string{found.name}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

var getNameInfo struct {
	once sync.Once
	proc *windows.LazyProc
}

func nameOf(ip netip.Addr) (string, error) {
	if err := winsockStarted(); err != nil {
		return "", err
	}
	getNameInfo.once.Do(func() { getNameInfo.proc = windows.NewLazySystemDLL("ws2_32.dll").NewProc("GetNameInfoW") })
	var (
		address unsafe.Pointer
		length  uintptr
	)
	if ip.Is4() {
		raw := windows.RawSockaddrInet4{Family: windows.AF_INET, Addr: ip.As4()}
		address, length = unsafe.Pointer(&raw), unsafe.Sizeof(raw)
	} else {
		raw := windows.RawSockaddrInet6{Family: windows.AF_INET6, Addr: ip.As16()}
		address, length = unsafe.Pointer(&raw), unsafe.Sizeof(raw)
	}
	var host [1025]uint16     // NI_MAXHOST
	const nameRequired = 0x04 // NI_NAMEREQD
	result, _, _ := getNameInfo.proc.Call(uintptr(address), length, uintptr(unsafe.Pointer(&host[0])), uintptr(len(host)), 0, 0, nameRequired)
	if result != 0 {
		return "", windows.Errno(result)
	}
	return windows.UTF16ToString(host[:]), nil
}
