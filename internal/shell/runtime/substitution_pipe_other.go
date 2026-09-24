//go:build !windows

package runtime

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
)

// substitutionPipe is the path `>(cmd)` hands its consumer: a FIFO in a private temporary
// directory, which a program opens by name and writes into while the command reads.
type substitutionPipe struct {
	path string
	dir  string
	// accepted is set once the command's side is open, after which abandon has nothing to do.
	accepted atomic.Bool
}

func newSubstitutionPipe() (*substitutionPipe, error) {
	dir, err := os.MkdirTemp("", "nemosh-fifo-*")
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		_ = os.Remove(dir)
		return nil, fmt.Errorf("mkfifo %s: %w", path, err)
	}
	return &substitutionPipe{path: path, dir: dir}, nil
}

// accept waits for the consumer to open the FIFO and answers with what it writes.
func (p *substitutionPipe) accept() (io.ReadCloser, error) {
	file, err := os.OpenFile(p.path, os.O_RDONLY, 0)
	p.accepted.Store(true)
	return file, err
}

// abandon lets accept return when the consumer never opened the path, by opening it for
// writing here and closing it again, so the command reads an empty input and ends. The open
// blocks until the command's side opens too, which it is about to: a non-blocking one fails
// when it comes first, and then the command's open would wait for ever.
func (p *substitutionPipe) abandon() {
	if p.accepted.Load() {
		return
	}
	if file, err := os.OpenFile(p.path, os.O_WRONLY, 0); err == nil {
		_ = file.Close()
	}
}

// remove takes the FIFO and its directory away once the command has finished with them.
func (p *substitutionPipe) remove() {
	_ = os.Remove(p.path)
	_ = os.Remove(p.dir)
}
