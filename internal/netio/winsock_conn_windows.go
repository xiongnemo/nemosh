package netio

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/windows"
)

// A Winsock connection. Each direction has an operation of its own -- an OVERLAPPED, its event
// and its buffer description, on the heap and the connection's -- so nc can read and write at
// once. An operation waits for its own event or the connection's closing one, the deadline its
// timeout; one that times out or is closed under is cancelled with CancelIoEx and then always
// waited for, WSAGetOverlappedResult with wait, because the kernel writes into the OVERLAPPED
// until it says it is done. Close waits for any operation under way to end before the socket goes.

type operation struct {
	overlapped windows.Overlapped
	buffer     windows.WSABuf
	deadline   atomic.Int64 // UnixNano, 0 for none
}

type winConn struct {
	socket      windows.Handle
	read, write *operation
	closing     windows.Handle // manual reset, set by Close
	closed      atomic.Bool
	// using is held shared by each operation under way and alone by Close, which so waits for
	// every one to end before the socket goes, and refuses any that would begin after.
	using         sync.RWMutex
	closeOnce     sync.Once
	local, remote string
}

func newConn(socket windows.Handle) (*winConn, error) {
	conn := &winConn{socket: socket, read: &operation{}, write: &operation{}}
	var err error
	for _, event := range []*windows.Handle{&conn.read.overlapped.HEvent, &conn.write.overlapped.HEvent, &conn.closing} {
		if *event, err = windows.CreateEvent(nil, 1, 0, nil); err != nil {
			conn.socket = 0 // the caller's to close
			conn.release()
			return nil, err
		}
	}
	if address, err := windows.Getsockname(socket); err == nil {
		conn.local = addressText(address)
	}
	if address, err := windows.Getpeername(socket); err == nil {
		conn.remote = addressText(address)
	}
	return conn, nil
}

// begin counts an operation under way, or refuses one after Close.
func (c *winConn) begin() bool {
	c.using.RLock()
	if c.closed.Load() {
		c.using.RUnlock()
		return false
	}
	return true
}

// await waits for op, started and pending, to end: done, timed out, or closed under.
func (c *winConn) await(op *operation) (uint32, error) {
	timeout := uint32(windows.INFINITE)
	if deadline := op.deadline.Load(); deadline != 0 {
		remaining := time.Until(time.Unix(0, deadline))
		timeout = uint32(max(remaining.Milliseconds(), 0))
	}
	which, _ := windows.WaitForMultipleObjects([]windows.Handle{op.overlapped.HEvent, c.closing}, false, timeout)
	cancelled := which != windows.WAIT_OBJECT_0
	if cancelled {
		_ = windows.CancelIoEx(c.socket, &op.overlapped)
	}
	var done, flags uint32
	err := windows.WSAGetOverlappedResult(c.socket, &op.overlapped, &done, true, &flags)
	switch {
	case err == nil:
		return done, nil
	case !cancelled:
		return 0, err
	case which == windows.WAIT_OBJECT_0+1:
		return 0, ErrClosed
	}
	return 0, ErrTimeout
}

func (c *winConn) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	if !c.begin() {
		return 0, ErrClosed
	}
	defer c.using.RUnlock()
	op := c.read
	_ = windows.ResetEvent(op.overlapped.HEvent)
	op.buffer = windows.WSABuf{Len: uint32(min(len(buffer), 1<<30)), Buf: &buffer[0]}
	var received, flags uint32
	err := windows.WSARecv(c.socket, &op.buffer, 1, &received, &flags, &op.overlapped, nil)
	if errors.Is(err, windows.ERROR_IO_PENDING) {
		received, err = c.await(op)
	}
	switch {
	case err != nil:
		return 0, c.closedOr(err)
	case received == 0:
		return 0, io.EOF
	}
	return int(received), nil
}

func (c *winConn) Write(buffer []byte) (int, error) {
	if !c.begin() {
		return 0, ErrClosed
	}
	defer c.using.RUnlock()
	op := c.write
	written := 0
	for written < len(buffer) {
		_ = windows.ResetEvent(op.overlapped.HEvent)
		op.buffer = windows.WSABuf{Len: uint32(min(len(buffer)-written, 1<<30)), Buf: &buffer[written]}
		var sent uint32
		err := windows.WSASend(c.socket, &op.buffer, 1, &sent, 0, &op.overlapped, nil)
		if errors.Is(err, windows.ERROR_IO_PENDING) {
			sent, err = c.await(op)
		}
		if err != nil {
			return written, c.closedOr(err)
		}
		written += int(sent)
	}
	return written, nil
}

// closedOr is ErrClosed for a socket Close took from under an operation, and err otherwise.
func (c *winConn) closedOr(err error) error {
	if c.closed.Load() {
		return ErrClosed
	}
	return err
}

func (c *winConn) CloseWrite() error {
	if c.closed.Load() {
		return ErrClosed
	}
	return windows.Shutdown(c.socket, windows.SHUT_WR)
}

func (c *winConn) SetReadDeadline(deadline time.Time) error {
	c.read.deadline.Store(unixNanoOrZero(deadline))
	return nil
}

func (c *winConn) SetWriteDeadline(deadline time.Time) error {
	c.write.deadline.Store(unixNanoOrZero(deadline))
	return nil
}

func unixNanoOrZero(deadline time.Time) int64 {
	if deadline.IsZero() {
		return 0
	}
	return deadline.UnixNano()
}

func (c *winConn) LocalAddr() string  { return c.local }
func (c *winConn) RemoteAddr() string { return c.remote }

// Close ends any operation under way, waits for it to, and then lets the socket go.
func (c *winConn) Close() error {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		_ = windows.SetEvent(c.closing)
		c.using.Lock()
		defer c.using.Unlock()
		c.release()
	})
	return nil
}

func (c *winConn) release() {
	if c.socket != windows.InvalidHandle && c.socket != 0 {
		_ = windows.Closesocket(c.socket)
	}
	for _, event := range []windows.Handle{c.read.overlapped.HEvent, c.write.overlapped.HEvent, c.closing} {
		if event != 0 {
			_ = windows.CloseHandle(event)
		}
	}
}
