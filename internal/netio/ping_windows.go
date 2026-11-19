package netio

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Echo is an ICMP echo by IcmpSendEcho2 or Icmp6SendEcho2, which any process may send, where a raw
// socket needs an administrator. The round trip is measured around the call: Windows reports it
// in whole milliseconds, and ping prints it to the microsecond.

// EchoReply is what came back: from where, its IP_STATUS -- 0 for an echo reply, 11003 for a host
// unreachable, 11013 for a TTL expired and the rest -- the TTL it arrived with, -1 where Windows
// does not say, as of an IPv6 reply, and the round trip.
type EchoReply struct {
	From   netip.Addr
	Status uint32
	TTL    int
	RTT    time.Duration
}

// EchoTimedOut is IP_REQ_TIMED_OUT, the status of an echo nothing answered.
const EchoTimedOut = 11010

var icmp struct {
	once                                 sync.Once
	create4, create6, send4, send6, done *windows.LazyProc
	// counter and frequency are QueryPerformanceCounter's: Go's clock on Windows moves at the
	// timer's tick, and an echo to the loopback took 0s by it.
	counter, frequency *windows.LazyProc
}

// icmpEchoReply is ICMP_ECHO_REPLY, whose pointers make it 28 bytes on 386 and 40 on amd64 and
// arm64; uintptr gives Go the same layout as C on each.
type icmpEchoReply struct {
	Address, Status, RoundTripTime uint32
	DataSize, Reserved             uint16
	Data                           uintptr
	TTL, TOS, Flags, OptionsSize   uint8
	OptionsData                    uintptr
}

// Pinger sends echoes of one family through one ICMP handle.
type Pinger struct {
	handle windows.Handle
	six    bool
}

// NewPinger opens an ICMP handle, for IPv6 when six is set.
func NewPinger(six bool) (*Pinger, error) {
	icmp.once.Do(func() {
		iphlpapi := windows.NewLazySystemDLL("iphlpapi.dll")
		icmp.create4, icmp.create6 = iphlpapi.NewProc("IcmpCreateFile"), iphlpapi.NewProc("Icmp6CreateFile")
		icmp.send4, icmp.send6 = iphlpapi.NewProc("IcmpSendEcho2"), iphlpapi.NewProc("Icmp6SendEcho2")
		icmp.done = iphlpapi.NewProc("IcmpCloseHandle")
		kernel32 := windows.NewLazySystemDLL("kernel32.dll")
		icmp.counter, icmp.frequency = kernel32.NewProc("QueryPerformanceCounter"), kernel32.NewProc("QueryPerformanceFrequency")
	})
	create := icmp.create4
	if six {
		create = icmp.create6
	}
	handle, _, err := create.Call()
	if windows.Handle(handle) == windows.InvalidHandle {
		return nil, err
	}
	return &Pinger{handle: windows.Handle(handle), six: six}, nil
}

// Close lets the handle go.
func (p *Pinger) Close() { icmp.done.Call(uintptr(p.handle)) }

// Echo sends size bytes to to with the TTL given, 0 for the system's, and waits up to timeout.
func (p *Pinger) Echo(to netip.Addr, size, ttl int, timeout time.Duration) (EchoReply, error) {
	data := make([]byte, max(size, 1))
	for index := range data {
		data[index] = byte(index)
	}
	options := struct {
		TTL, TOS, Flags, OptionsSize uint8
		OptionsData                  uintptr
	}{TTL: uint8(ttl)}
	reply := make([]byte, 256+size)
	milliseconds := uintptr(max(timeout.Milliseconds(), 1))
	// No options for the system's TTL: Icmp6SendEcho2 answers a hop limit of 0 with a general failure.
	// A Pointer, converted only in the call's arguments, where the runtime keeps what it points at.
	var optionsPointer unsafe.Pointer
	if ttl > 0 {
		optionsPointer = unsafe.Pointer(&options)
	}
	var count uintptr
	var err error
	var start, end, frequency int64
	icmp.counter.Call(uintptr(unsafe.Pointer(&start)))
	if p.six {
		var source, destination windows.RawSockaddrInet6
		source.Family, destination.Family = windows.AF_INET6, windows.AF_INET6
		destination.Addr = to.As16()
		count, _, err = icmp.send6.Call(uintptr(p.handle), 0, 0, 0, uintptr(unsafe.Pointer(&source)), uintptr(unsafe.Pointer(&destination)),
			uintptr(unsafe.Pointer(&data[0])), uintptr(size), uintptr(optionsPointer), uintptr(unsafe.Pointer(&reply[0])), uintptr(len(reply)), milliseconds)
	} else {
		address := to.As4()
		count, _, err = icmp.send4.Call(uintptr(p.handle), 0, 0, 0, uintptr(binary.LittleEndian.Uint32(address[:])),
			uintptr(unsafe.Pointer(&data[0])), uintptr(size), uintptr(optionsPointer), uintptr(unsafe.Pointer(&reply[0])), uintptr(len(reply)), milliseconds)
	}
	icmp.counter.Call(uintptr(unsafe.Pointer(&end)))
	icmp.frequency.Call(uintptr(unsafe.Pointer(&frequency)))
	elapsed := time.Duration(float64(end-start) / float64(max(frequency, 1)) * float64(time.Second))
	if count == 0 {
		var errno windows.Errno
		if errors.As(err, &errno) && errno >= 11000 && errno < 12000 {
			return EchoReply{Status: uint32(errno), TTL: -1}, nil
		}
		return EchoReply{}, err
	}
	if p.six {
		// ICMPV6_ECHO_REPLY: an IPV6_ADDRESS_EX, packed, 26 bytes -- its address from the sixth --
		// then the status at 28 and the time at 32. It has no TTL.
		return EchoReply{From: netip.AddrFrom16([16]byte(reply[6:22])), Status: binary.LittleEndian.Uint32(reply[28:]), TTL: -1, RTT: elapsed}, nil
	}
	answer := (*icmpEchoReply)(unsafe.Pointer(&reply[0]))
	var from [4]byte
	binary.LittleEndian.PutUint32(from[:], answer.Address)
	return EchoReply{From: netip.AddrFrom4(from), Status: answer.Status, TTL: int(answer.TTL), RTT: elapsed}, nil
}
