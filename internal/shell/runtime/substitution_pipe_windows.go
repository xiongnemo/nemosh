//go:build windows

package runtime

import (
	"fmt"
	"io"
	"os"
	"sync/atomic"

	"golang.org/x/sys/windows"
)

var substitutionPipes atomic.Uint64

// substitutionPipe is the path `>(cmd)` hands its consumer: a named pipe, the one thing on
// Windows a program can open by name and write into while another reads. Measured before
// choosing it: Go's and Python's write opens both reach it, with create and truncate asked
// for; duplex, so an opener asking to read as well is not refused.
type substitutionPipe struct {
	path   string
	handle windows.Handle
}

func newSubstitutionPipe() (*substitutionPipe, error) {
	path := fmt.Sprintf("//./pipe/nemosh-%d-%d", os.Getpid(), substitutionPipes.Add(1))
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateNamedPipe(name,
		windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_FIRST_PIPE_INSTANCE,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT|windows.PIPE_REJECT_REMOTE_CLIENTS,
		1, 64*1024, 64*1024, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", path, err)
	}
	return &substitutionPipe{path: path, handle: handle}, nil
}

// accept waits for the consumer to open the pipe and answers with what it writes.
func (p *substitutionPipe) accept() (io.ReadCloser, error) {
	// ERROR_NO_DATA is a consumer that opened, wrote and closed before this asked: what it
	// wrote is still in the pipe, and reading drains it before the end of input -- measured.
	// Taking it for an empty input lost the whole of `echo hi > >(cat)` whenever the writer
	// was quicker than this goroutine, which on a fast machine is always.
	err := windows.ConnectNamedPipe(p.handle, nil)
	if err != nil && err != windows.ERROR_PIPE_CONNECTED && err != windows.ERROR_NO_DATA {
		_ = windows.CloseHandle(p.handle)
		return nil, fmt.Errorf("connect %s: %w", p.path, err)
	}
	return os.NewFile(uintptr(p.handle), p.path), nil
}

// abandon lets accept return when the consumer never opened the path -- `echo >(cat)` only
// prints it -- by opening it here and closing it again, so the command reads an empty input
// and ends, as it does in bash. When the consumer did open it the open fails, harmlessly.
func (p *substitutionPipe) abandon() {
	if file, err := os.OpenFile(p.path, os.O_WRONLY, 0); err == nil {
		_ = file.Close()
	}
}

// remove has nothing to remove: a named pipe goes with its last handle.
func (p *substitutionPipe) remove() {}
