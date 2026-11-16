package netio

import (
	"errors"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

// A Winsock listener: AcceptEx into a socket made for each connection, waiting on its event or
// the listener's closing one, as a connection's operations wait.

type winListener struct {
	socket    windows.Handle
	family    int32
	accept    *operation
	closing   windows.Handle
	closed    atomic.Bool
	using     sync.RWMutex
	closeOnce sync.Once
	address   string
}

// Listen waits for connections on host's port; port 0 is one the system picks.
func Listen(host, port string) (Listener, error) {
	addresses, err := resolve(host, port)
	if err != nil {
		return nil, err
	}
	address := addresses[0]
	socket, err := newSocket(familyOf(address))
	if err != nil {
		return nil, err
	}
	listener := &winListener{socket: socket, family: familyOf(address), accept: &operation{}}
	fail := func(err error) (Listener, error) {
		listener.release()
		return nil, err
	}
	// Nothing else may take the port while it is this listener's.
	const exclusiveAddressUse = ^int(4) // SO_EXCLUSIVEADDRUSE, ~SO_REUSEADDR
	if err := windows.SetsockoptInt(socket, windows.SOL_SOCKET, exclusiveAddressUse, 1); err != nil {
		return fail(err)
	}
	if err := windows.Bind(socket, address); err != nil {
		return fail(err)
	}
	if err := windows.Listen(socket, windows.SOMAXCONN); err != nil {
		return fail(err)
	}
	for _, event := range []*windows.Handle{&listener.accept.overlapped.HEvent, &listener.closing} {
		if *event, err = windows.CreateEvent(nil, 1, 0, nil); err != nil {
			return fail(err)
		}
	}
	if bound, err := windows.Getsockname(socket); err == nil {
		listener.address = addressText(bound)
	}
	return listener, nil
}

func (l *winListener) Accept() (Conn, error) {
	l.using.RLock()
	defer l.using.RUnlock()
	if l.closed.Load() {
		return nil, ErrClosed
	}
	accepted, err := newSocket(l.family)
	if err != nil {
		return nil, err
	}
	op := l.accept
	_ = windows.ResetEvent(op.overlapped.HEvent)
	// Room for both addresses, each sizeof(SOCKADDR_STORAGE)+16 as AcceptEx asks.
	const addressRoom = 128 + 16
	var addresses [2 * addressRoom]byte
	var received uint32
	err = windows.AcceptEx(l.socket, accepted, &addresses[0], 0, addressRoom, addressRoom, &received, &op.overlapped)
	if errors.Is(err, windows.ERROR_IO_PENDING) {
		which, _ := windows.WaitForMultipleObjects([]windows.Handle{op.overlapped.HEvent, l.closing}, false, windows.INFINITE)
		if which != windows.WAIT_OBJECT_0 {
			_ = windows.CancelIoEx(l.socket, &op.overlapped)
		}
		var flags uint32
		err = windows.WSAGetOverlappedResult(l.socket, &op.overlapped, &received, true, &flags)
		if err != nil && which != windows.WAIT_OBJECT_0 {
			err = ErrClosed
		}
	}
	if err != nil {
		_ = windows.Closesocket(accepted)
		return nil, err
	}
	// So getpeername, shutdown and the rest know the socket is a connection of the listener's.
	const updateAcceptContext = 0x700B // SO_UPDATE_ACCEPT_CONTEXT
	if err := windows.Setsockopt(accepted, windows.SOL_SOCKET, updateAcceptContext,
		(*byte)(unsafe.Pointer(&l.socket)), int32(unsafe.Sizeof(l.socket))); err != nil {
		_ = windows.Closesocket(accepted)
		return nil, err
	}
	conn, err := newConn(accepted)
	if err != nil {
		_ = windows.Closesocket(accepted)
		return nil, err
	}
	return conn, nil
}

func (l *winListener) Addr() string { return l.address }

// Close ends an Accept under way, waits for it to, and lets the socket go.
func (l *winListener) Close() error {
	l.closeOnce.Do(func() {
		l.closed.Store(true)
		if l.closing != 0 {
			_ = windows.SetEvent(l.closing)
		}
		l.using.Lock()
		defer l.using.Unlock()
		l.release()
	})
	return nil
}

func (l *winListener) release() {
	_ = windows.Closesocket(l.socket)
	for _, event := range []windows.Handle{l.accept.overlapped.HEvent, l.closing} {
		if event != 0 {
			_ = windows.CloseHandle(event)
		}
	}
}
