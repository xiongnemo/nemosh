package runtime

import (
	"io"
	"syscall"
)

// `read -t 0` reads nothing and answers whether it could: 0 when input is waiting or has
// ended, 1 when a read would wait, as busybox and bash both answer. It waited no time for a
// line and reported the timeout, 142, whatever was there -- so `read -t 0 && ...`, the way
// to ask whether anything was piped in, always said no.

// inputReady answers for a reader: one behind a descriptor is asked, and one in memory never
// waits.
//
// The descriptor is reached through SyscallConn rather than Fd. On Unix, Fd puts a pipe back
// into blocking mode, and a read blocked on it could then no longer be ended by closing the
// pipe, which is how a pipeline is taken down.
func inputReady(input io.Reader) bool {
	conn, ok := input.(syscall.Conn)
	if !ok {
		return true
	}
	raw, err := conn.SyscallConn()
	if err != nil {
		return true
	}
	ready := true
	if err := raw.Control(func(descriptor uintptr) { ready = handleReady(descriptor) }); err != nil {
		return true
	}
	return ready
}
