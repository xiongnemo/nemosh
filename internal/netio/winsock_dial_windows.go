package netio

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Dial connects to host's port, each address the name has in turn, giving up after timeout or
// when ctx is cancelled. A name is resolved first, by GetAddrInfoW in a goroutine of its own,
// which nothing can end early -- Go's net cannot either -- so a cancelled dial returns at once
// and the lookup finishes alone.
func Dial(ctx context.Context, host, port string, timeout time.Duration) (Conn, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	type lookup struct {
		addresses []windows.Sockaddr
		err       error
	}
	looked := make(chan lookup, 1)
	go func() {
		addresses, err := resolve(host, port)
		looked <- lookup{addresses, err}
	}()
	var found lookup
	select {
	case found = <-looked:
	case <-ctx.Done():
		return nil, dialError(host, port, ctx.Err())
	}
	if found.err != nil {
		return nil, found.err
	}
	var last error
	for _, address := range found.addresses {
		conn, err := connect(ctx, address)
		if err == nil {
			return conn, nil
		}
		last = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, dialError(host, port, last)
}

func dialError(host, port string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		err = ErrTimeout
	}
	return fmt.Errorf("dial tcp %s: %w", JoinHostPort(host, port), err)
}

// connect is one address, by ConnectEx on a socket bound to the unspecified address.
func connect(ctx context.Context, address windows.Sockaddr) (Conn, error) {
	socket, err := newSocket(familyOf(address))
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = windows.Closesocket(socket)
		}
	}()
	if err := windows.Bind(socket, anyAddress(familyOf(address))); err != nil {
		return nil, err
	}
	var overlapped windows.Overlapped
	if overlapped.HEvent, err = windows.CreateEvent(nil, 1, 0, nil); err != nil {
		return nil, err
	}
	defer windows.CloseHandle(overlapped.HEvent)
	cancelled, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(cancelled)
	stop := context.AfterFunc(ctx, func() { _ = windows.SetEvent(cancelled) })
	defer stop()
	failFastOnLoopback(socket, address)
	err = windows.ConnectEx(socket, address, nil, 0, nil, &overlapped)
	if errors.Is(err, windows.ERROR_IO_PENDING) {
		which, _ := windows.WaitForMultipleObjects([]windows.Handle{overlapped.HEvent, cancelled}, false, windows.INFINITE)
		if which != windows.WAIT_OBJECT_0 {
			_ = windows.CancelIoEx(socket, &overlapped)
		}
		var done, flags uint32
		err = windows.WSAGetOverlappedResult(socket, &overlapped, &done, true, &flags)
		if err != nil && which != windows.WAIT_OBJECT_0 {
			err = ctx.Err()
		}
	}
	if err != nil {
		return nil, err
	}
	// So getpeername, shutdown and the rest know the socket is connected.
	const updateConnectContext = 0x7010 // SO_UPDATE_CONNECT_CONTEXT
	if err := windows.Setsockopt(socket, windows.SOL_SOCKET, updateConnectContext, nil, 0); err != nil {
		return nil, err
	}
	conn, err := newConn(socket)
	if err != nil {
		return nil, err
	}
	keep = true
	return conn, nil
}

// failFastOnLoopback makes a connection to this machine that nothing answers fail at once, as Go's
// net makes it: Windows otherwise sends the SYN twice more after the refusal, and a refused
// connection to 127.0.0.1 took two seconds. SIO_TCP_INITIAL_RTO with no retransmission of the
// SYN, Windows 10 1709 and later; where it is refused, the two seconds stay.
func failFastOnLoopback(socket windows.Handle, address windows.Sockaddr) {
	loopback := false
	switch address := address.(type) {
	case *windows.SockaddrInet4:
		loopback = address.Addr[0] == 127
	case *windows.SockaddrInet6:
		loopback = address.Addr == [16]byte{15: 1}
	}
	if !loopback {
		return
	}
	parameters := struct {
		rtt                   uint16
		maxSynRetransmissions uint8
	}{rtt: ^uint16(0), maxSynRetransmissions: ^uint8(1)}
	const initialRTO = 0x98000011 // SIO_TCP_INITIAL_RTO: IOC_IN | IOC_VENDOR | 17
	var returned uint32
	_ = windows.WSAIoctl(socket, initialRTO, (*byte)(unsafe.Pointer(&parameters)), uint32(unsafe.Sizeof(parameters)), nil, 0, &returned, nil, 0)
}
