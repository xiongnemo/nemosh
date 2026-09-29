package applets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// xargsPool is -P N's PROGs running side by side, as busybox's xargs runs them: no more than N
// at once, 64 for -P 0 as busybox-w32 has it, and their writes to one stdout or stderr each
// whole, as separate processes' writes to one descriptor are. A PROG that ends 255 stops the
// ones not yet begun, and xargs ends when the running ones do.
type xargsPool struct {
	slots chan struct{}
	group sync.WaitGroup
	mutex sync.Mutex
	stop  error
}

func newXargsPool(procs int) *xargsPool {
	if procs == 0 {
		procs = 64
	}
	return &xargsPool{slots: make(chan struct{}, procs)}
}

// start runs a PROG in a slot of its own, once one is free, unless one before it has stopped
// xargs.
func (p *xargsPool) start(x *xargsRun, applet Applet, line []string) error {
	p.slots <- struct{}{}
	p.mutex.Lock()
	stop := p.stop
	p.mutex.Unlock()
	if stop != nil {
		<-p.slots
		return stop
	}
	p.group.Add(1)
	go func() {
		defer func() {
			<-p.slots
			p.group.Done()
		}()
		err := applet.Run(x.ctx, line[1:], bytes.NewReader(nil), x.stdout, x.stderr)
		if stop := x.settle(line[0], err); stop != nil {
			p.mutex.Lock()
			if p.stop == nil {
				p.stop = stop
			}
			p.mutex.Unlock()
		}
	}()
	return nil
}

// wait is the end of the running PROGs, and what stopped xargs, if anything did.
func (p *xargsPool) wait() error {
	p.group.Wait()
	return p.stop
}

// settle is busybox's xargs_exec's reckoning of a PROG's status: 255 stops xargs with 124, an
// interrupt stops it too, and any other failure is remembered for 123 at the end.
func (x *xargsRun) settle(name string, err error) error {
	if x.ctx.Err() != nil {
		return context.Cause(x.ctx)
	}
	if errors.Is(err, ErrInterrupt) {
		return err
	}
	switch status := xargsStatus(name, err, x.stderr); {
	case status == 255:
		return ExitStatusMessage(124, fmt.Errorf("%s: exited with status 255; aborting", name))
	case status != 0:
		x.mutex.Lock()
		x.failed = true
		x.mutex.Unlock()
	}
	return nil
}

// lockedWriter is a stdout or stderr the PROGs of -P share, each Write whole under one lock
// for both, for 2>&1 has them one stream. It answers where the terminal is, as the writer it
// wraps does: a wrapper that hides that has stopped ls laying out columns before.
type lockedWriter struct {
	mutex  *sync.Mutex
	writer io.Writer
}

func (w lockedWriter) Write(buffer []byte) (int, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	return w.writer.Write(buffer)
}

func (w lockedWriter) TerminalFile() *os.File { return stdoutFile(w.writer) }
