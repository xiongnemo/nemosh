package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// Background jobs as processes, step two of docs/design/background-processes.md: behind
// NEMOSH_JOBS=process, a `cmd &` is this binary started as `nemosh --job <handle>`, given
// the job's state over an inherited pipe (job_state.go). `$!` is its pid; `wait` and
// `kill` take the pid as they take `%N`. Off unless asked for, because the goroutine is
// still what every other test and every user runs.
//
// Not yet: descriptors other than 0, 1 and 2 (a job that writes to a `3>file` of the
// shell's), the Job Object, and signals delivered to the child's traps. Those are steps
// three and four.

// processJobsEnabled reports NEMOSH_JOBS=process in the shell's environment, for a shell
// whose applets are the ones a child of this binary would have. A runtime given its own
// registry -- a test's applet that answers over a channel -- cannot be reproduced in
// another process, so its jobs stay goroutines.
func (r Runtime) processJobsEnabled() bool {
	value, _ := r.env.LookupEnv("NEMOSH_JOBS")
	return value == "process" && r.registry.IsDefault()
}

// jobExecutable is the program a job process runs: this binary. A variable so a test,
// whose binary is not nemosh, can point it at one that is.
var jobExecutable = os.Executable

// launchProcessJob starts node as a job process and records it.
func (r Runtime) launchProcessJob(node programNode) lineResult {
	state := r.captureJobState(node)
	// The job's own first line, where `$LINENO` counts from; the shell's current line is
	// the command before it.
	if line := firstCommandLine(node); line > 0 {
		state.Line = line
	}
	// A job starts with no traps, as the goroutine's worker does.
	state.Traps = map[string]string{}
	data, err := json.Marshal(state)
	if err != nil {
		return r.jobLaunchFailure(err)
	}
	executable, err := jobExecutable()
	if err != nil {
		return r.jobLaunchFailure(err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return r.jobLaunchFailure(err)
	}
	// Under the scope's context, so a scope that is torn down -- a subshell ending with a
	// job still running -- ends its process as it ends a goroutine. The root scope is not
	// cancelled when the shell exits, so a job started at the top can outlive it, as both
	// references' jobs do.
	command := exec.CommandContext(r.jobScope.ctx, executable)
	argument, err := prepareJobCommand(command, reader)
	if err != nil {
		return r.jobLaunchFailure(errors.Join(err, reader.Close(), writer.Close()))
	}
	command.Args = []string{executable, "--job", argument}
	command.Env = r.env.Environ()
	if directory, err := r.nativeWorkingDirectory(); err == nil {
		command.Dir = directory
	}
	// No stdin, as POSIX gives an asynchronous list without job control and busybox gives
	// every background job.
	command.Stdout, command.Stderr = r.streams.Stdout, r.streams.Stderr
	if err := command.Start(); err != nil {
		return r.jobLaunchFailure(errors.Join(err, reader.Close(), writer.Close()))
	}
	_ = reader.Close()
	go func() {
		_, _ = writer.Write(data)
		_ = writer.Close()
	}()
	record, err := r.jobScope.registerCancellable(func() { _ = command.Process.Kill() })
	if err != nil {
		_ = command.Process.Kill()
		return r.jobLaunchFailure(err)
	}
	record.pid = command.Process.Pid
	r.vars["!"] = strconv.Itoa(record.pid)
	r.markVarMutation("!")
	if r.interactive.session {
		fmt.Fprintf(r.streams.Stderr, "[%d] %d\n", record.id, record.pid)
	}
	go func() {
		err := command.Wait()
		status := 0
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			status = jobExitStatus(uint32(exit.ExitCode()))
		} else if err != nil {
			status = 1
		}
		r.jobScope.complete(record, status)
	}()
	return lineResult{}
}

// lookupPID finds the job whose process this is, for a `wait` or `kill` given a pid.
func (s *jobScope) lookupPID(pid int) (jobID, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, record := range s.records {
		if record.pid != 0 && record.pid == pid {
			return id, true
		}
	}
	return 0, false
}

func (r Runtime) jobLaunchFailure(err error) lineResult {
	fmt.Fprintf(r.streams.Stderr, "nemosh: starting a background job: %v\n", err)
	return lineResult{status: 1}
}

// jobExitStatus reads a job process's exit code as a shell status. A code with the signal
// in its top byte is how busybox's kill and proc.Terminate end a process, `n << 24`, and it
// is 128+n as a status, as a wait there reports it.
func jobExitStatus(code uint32) int {
	if code >= 1<<24 && code&0xffffff == 0 {
		return 128 + int(code>>24)
	}
	return int(code & 0xff)
}

// RunJob is the child's side, `nemosh --job`: the state read, the job run, its status.
func (r *Runtime) RunJob(ctx context.Context, data []byte) int {
	var state jobState
	if err := json.Unmarshal(data, &state); err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: job state: %v\n", err)
		return 2
	}
	program, err := r.restoreJobState(ctx, state)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: job: %v\n", err)
		return 2
	}
	status, _ := r.executePrepared(ctx, program)
	r.CloseBatch(status)
	return status
}

// firstCommandLine is the source line of the first command in node, or 0 when it has none
// the parser numbered.
func firstCommandLine(node programNode) int {
	switch value := node.(type) {
	case listNode:
		for _, item := range value.value.items {
			for _, pipeline := range item.value.pipelines {
				for _, command := range pipeline.commands {
					if line := firstLineOfCommand(command); line > 0 {
						return line
					}
				}
			}
		}
	case backgroundNode:
		return firstCommandLine(value.value)
	case loopNode:
		return value.line
	case caseNode:
		return value.line
	case ifNode:
		return firstCommandLine(listNode{value: value.condition})
	}
	return 0
}

func firstLineOfCommand(command commandNode) int {
	switch value := command.(type) {
	case simpleCommand:
		return value.line
	case braceGroup:
		if len(value.body.program) > 0 {
			return firstCommandLine(value.body.program[0])
		}
	case subshellCommand:
		if len(value.body.program) > 0 {
			return firstCommandLine(value.body.program[0])
		}
	}
	return 0
}
