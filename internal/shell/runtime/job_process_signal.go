package runtime

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
)

// Signals for a job that is a process, step three of docs/design/background-processes.md.
// KILL ends the job's whole tree -- its Job Object on Windows, its process group elsewhere
// -- so the programs the job started go with it. Every other signal is one byte on a
// control pipe the child inherits. The child then does with it what signal_inbox.go says:
// runs its trap, ignores it, or ends by it. It ends by the signal, not an exit status, so
// the parent can tell `kill` apart from `exit 143`.

// signalSender is the parent's end of a job process's control pipe.
type signalSender struct {
	mu   sync.Mutex
	pipe *os.File
}

// send reports whether the job has the signal. False is a job that has already gone, and
// the default action is the caller's.
func (s *signalSender) send(signal int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pipe == nil || signal <= 0 || signal > 255 {
		return false
	}
	_, err := s.pipe.Write([]byte{byte(signal)})
	return err == nil
}

func (s *signalSender) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pipe != nil {
		_ = s.pipe.Close()
		s.pipe = nil
	}
}

// control gives the child the read end of a control pipe, and answers with the sender and
// the handle the child finds its end under.
func (h *jobHandoff) control() (*signalSender, string, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, "", err
	}
	h.given = append(h.given, reader.Close)
	handle, err := h.give(reader)
	if err != nil {
		return nil, "", errors.Join(err, writer.Close())
	}
	return &signalSender{pipe: writer}, handle, nil
}

// receiveSignals is the child's end: each signal offered to the job's inbox, and the job
// ended by one no trap catches. It stops when the parent has gone, and the job carries on
// without it, as a job outlives its shell.
func (r Runtime) receiveSignals(control *os.File, end context.CancelCauseFunc) {
	defer control.Close()
	buffer := make([]byte, 1)
	for {
		if _, err := control.Read(buffer); err != nil {
			return
		}
		if signal := int(buffer[0]); !r.signals.offer(signal) {
			end(jobSignal(signal))
		}
	}
}

// jobOutcome reads how a job process ended: its status, and the signal that ended it, or 0.
func jobOutcome(command *exec.Cmd, err error) (int, int) {
	if command.ProcessState == nil {
		return 1, 0
	}
	status, signal := processOutcome(command.ProcessState)
	if err != nil && status == 0 && signal == 0 {
		// An error with a clean exit is the copy of its output failing.
		return 1, 0
	}
	return status, signal
}

// noteExitSignal records the signal a job process ended by, unless `kill` has already said
// which: a TERM the job did not answer is still a TERM, however it was then ended.
func (s *jobScope) noteExitSignal(record *jobRecord, signal int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if record.signal == 0 {
		record.signal = signal
	}
}
