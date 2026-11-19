package netio

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Interface is one network adapter as GetAdaptersAddresses describes it, which is what hostname
// -I and ip read: its index, the name Windows shows for it -- Ethernet, Wi-Fi -- its hardware
// address, its MTU, whether it is up and whether it is the loopback, its addresses with their
// prefix lengths, and its gateways.
type Interface struct {
	Index     int
	Name      string
	Hardware  []byte
	MTU       int
	Up        bool
	Loopback  bool
	Addresses []netip.Prefix
	Gateways  []netip.Addr
}

// Interfaces is every adapter, in the order Windows lists them. Go's net would answer this too,
// but its init starts Winsock in every process, which is why nothing here imports it.
func Interfaces() ([]Interface, error) {
	const includeGateways = 0x80 // GAA_FLAG_INCLUDE_GATEWAYS
	flags := uint32(windows.GAA_FLAG_SKIP_ANYCAST | windows.GAA_FLAG_SKIP_MULTICAST | windows.GAA_FLAG_SKIP_DNS_SERVER | includeGateways)
	size := uint32(15 << 10)
	var buffer []byte
	for {
		buffer = make([]byte, size)
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, flags, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&buffer[0])), &size)
		if err == nil {
			break
		}
		if !errors.Is(err, windows.ERROR_BUFFER_OVERFLOW) {
			return nil, err
		}
	}
	var interfaces []Interface
	for adapter := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buffer[0])); adapter != nil; adapter = adapter.Next {
		found := Interface{
			Index:    int(adapter.IfIndex),
			Name:     windows.UTF16PtrToString(adapter.FriendlyName),
			Hardware: slices.Clone(adapter.PhysicalAddress[:adapter.PhysicalAddressLength]),
			MTU:      int(adapter.Mtu),
			Up:       adapter.OperStatus == windows.IfOperStatusUp,
			Loopback: adapter.IfType == windows.IF_TYPE_SOFTWARE_LOOPBACK,
		}
		if found.Index == 0 {
			found.Index = int(adapter.Ipv6IfIndex)
		}
		for unicast := adapter.FirstUnicastAddress; unicast != nil; unicast = unicast.Next {
			if address, ok := addressOf(unicast.Address.Sockaddr); ok {
				found.Addresses = append(found.Addresses, netip.PrefixFrom(address, int(unicast.OnLinkPrefixLength)))
			}
		}
		for gateway := adapter.FirstGatewayAddress; gateway != nil; gateway = gateway.Next {
			if address, ok := addressOf(gateway.Address.Sockaddr); ok {
				found.Gateways = append(found.Gateways, address)
			}
		}
		interfaces = append(interfaces, found)
	}
	return interfaces, nil
}

// addressOf is the address a socket address holds, without its zone, which a prefix cannot have.
func addressOf(raw *syscall.RawSockaddrAny) (netip.Addr, bool) {
	if raw == nil {
		return netip.Addr{}, false
	}
	address, err := (*windows.RawSockaddrAny)(unsafe.Pointer(raw)).Sockaddr()
	if err != nil {
		return netip.Addr{}, false
	}
	switch address := address.(type) {
	case *windows.SockaddrInet4:
		return netip.AddrFrom4(address.Addr), true
	case *windows.SockaddrInet6:
		return netip.AddrFrom16(address.Addr), true
	}
	return netip.Addr{}, false
}

// LookupHost is the addresses host names, by the resolver Dial asks, each once and in its order.
// In a goroutine of its own, as Dial's lookup is, so a cancelled context returns at once.
func LookupHost(ctx context.Context, host string) ([]netip.Addr, error) {
	type answer struct {
		addresses []windows.Sockaddr
		err       error
	}
	answered := make(chan answer, 1)
	go func() {
		addresses, err := resolve(host, "0")
		answered <- answer{addresses, err}
	}()
	var found answer
	select {
	case found = <-answered:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if found.err != nil {
		return nil, found.err
	}
	var addresses []netip.Addr
	for _, socket := range found.addresses {
		var address netip.Addr
		switch socket := socket.(type) {
		case *windows.SockaddrInet4:
			address = netip.AddrFrom4(socket.Addr)
		case *windows.SockaddrInet6:
			address = netip.AddrFrom16(socket.Addr)
		}
		if address.IsValid() && !slices.Contains(addresses, address) {
			addresses = append(addresses, address)
		}
	}
	return addresses, nil
}
