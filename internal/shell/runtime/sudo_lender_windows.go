//go:build windows

package runtime

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"unsafe"

	"github.com/xiongnemo/nemosh/internal/sudo"
	"golang.org/x/sys/windows"
)

// sudoLender is the elevated nemosh of an SSH session listening for sudo: each request is run with
// this process's token, with the asker's streams, directory and environment; see package sudo.
//
// The pipe lets in this logon session alone, by its logon SID, at medium integrity so the session's
// unelevated shell can write to it, and no remote client. Any process in the session can ask, as
// any can under sudo with NOPASSWD: what the unelevated shell guards against is a mistake, and a
// program that refuses to run elevated, not a process of the same user that means harm.
type sudoLender struct {
	name       string
	listening  windows.Handle
	attributes *windows.SecurityAttributes
	launch     sudoLaunch
}

// sudoLaunch runs a request with the streams taken from the asker, the asker's process given so the
// command can end with it, and answers with the status and what to say.
type sudoLaunch func(request sudo.Request, stdio [3]*os.File, asker windows.Handle) (int, string)

func listenForSudo(name string, launch sudoLaunch) (*sudoLender, error) {
	attributes, err := sudoPipeSecurity()
	if err != nil {
		return nil, err
	}
	listening, err := createSudoPipe(name, attributes, windows.FILE_FLAG_FIRST_PIPE_INSTANCE)
	if err != nil {
		return nil, fmt.Errorf("listen for sudo: %w", err)
	}
	return &sudoLender{name: name, listening: listening, attributes: attributes, launch: launch}, nil
}

// serve takes askers one after another, each answered on a goroutine of its own, until a new
// instance of the pipe cannot be made.
func (l *sudoLender) serve() {
	pipe := l.listening
	for {
		if err := windows.ConnectNamedPipe(pipe, nil); err == nil || errors.Is(err, windows.ERROR_PIPE_CONNECTED) {
			go l.answer(pipe)
		} else {
			_ = windows.CloseHandle(pipe)
		}
		next, err := createSudoPipe(l.name, l.attributes, 0)
		if err != nil {
			return
		}
		pipe = next
	}
}

func (l *sudoLender) answer(pipe windows.Handle) {
	connection := os.NewFile(uintptr(pipe), l.name)
	defer connection.Close()
	status, message := l.run(pipe, connection)
	if sudo.WriteReply(connection, sudo.Reply{Status: int32(status), Message: message}) == nil {
		_ = windows.FlushFileBuffers(pipe)
	}
}

func (l *sudoLender) run(pipe windows.Handle, connection *os.File) (int, string) {
	request, err := sudo.ReadRequest(connection)
	if err != nil {
		return 1, err.Error()
	}
	var pid uint32
	if err := windows.GetNamedPipeClientProcessId(pipe, &pid); err != nil {
		return 1, fmt.Sprintf("cannot tell who asked: %v", err)
	}
	asker, err := windows.OpenProcess(windows.PROCESS_DUP_HANDLE|windows.SYNCHRONIZE, false, pid)
	if err != nil {
		return 1, fmt.Sprintf("cannot open the process that asked: %v", err)
	}
	defer windows.CloseHandle(asker)
	var stdio [3]*os.File
	for index, handle := range request.Handles {
		if handle == 0 {
			continue
		}
		var taken windows.Handle
		if err := windows.DuplicateHandle(asker, windows.Handle(handle), windows.CurrentProcess(), &taken, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
			return 1, fmt.Sprintf("cannot take sudo's streams: %v", err)
		}
		stdio[index] = os.NewFile(uintptr(taken), "")
		defer stdio[index].Close()
	}
	return l.launch(request, stdio, asker)
}

func createSudoPipe(name string, attributes *windows.SecurityAttributes, first uint32) (windows.Handle, error) {
	return windows.CreateNamedPipe(windows.StringToUTF16Ptr(name), windows.PIPE_ACCESS_DUPLEX|first,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT|windows.PIPE_REJECT_REMOTE_CLIENTS,
		windows.PIPE_UNLIMITED_INSTANCES, 4096, 4096, 0, attributes)
}

// endWithAsker ends process when the sudo that asked for it ends first, and closes its handle on the
// asker once done is closed. It looks a few times a second, so that it never waits on a handle that
// another goroutine could close under it.
func endWithAsker(asker windows.Handle, process *os.Process, done <-chan struct{}) {
	defer windows.CloseHandle(asker)
	for {
		select {
		case <-done:
			return
		default:
		}
		if event, _ := windows.WaitForSingleObject(asker, 200); event == windows.WAIT_OBJECT_0 {
			_ = process.Kill()
			return
		}
	}
}

// sudoPipeSecurity lets this logon session in, by its logon SID, and labels the pipe medium so the
// session's unelevated processes may write to it.
func sudoPipeSecurity() (*windows.SecurityAttributes, error) {
	groups, err := windows.GetCurrentProcessToken().GetTokenGroups()
	if err != nil {
		return nil, err
	}
	for _, group := range groups.AllGroups() {
		if group.Attributes&windows.SE_GROUP_LOGON_ID != windows.SE_GROUP_LOGON_ID {
			continue
		}
		descriptor, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;" + group.Sid.String() + ")S:(ML;;NW;;;ME)")
		if err != nil {
			return nil, err
		}
		return &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: descriptor}, nil
	}
	return nil, errors.New("this process's token has no logon SID")
}

// lendElevated runs a request as this elevated nemosh: the command through `exec "$@"`, or with
// sudo -s a shell, named as this one was so bash mode carries over, kept elevated by KeepVariable.
// A sudo that is gone takes its command with it.
func lendElevated(self, name string) sudoLaunch {
	return func(request sudo.Request, stdio [3]*os.File, asker windows.Handle) (int, string) {
		args := []string{name}
		if !request.Shell && len(request.Args) > 0 {
			args = append(append(args, "-c", `exec "$@"`, "sudo"), request.Args...)
		}
		command := &exec.Cmd{Path: self, Args: args, Dir: request.Dir, Env: append(request.Env, sudo.KeepVariable+"=keep")}
		if stdio[0] != nil {
			command.Stdin = stdio[0]
		}
		if stdio[1] != nil {
			command.Stdout = stdio[1]
		}
		if stdio[2] != nil {
			command.Stderr = stdio[2]
		}
		var watched windows.Handle
		if err := windows.DuplicateHandle(windows.CurrentProcess(), asker, windows.CurrentProcess(), &watched, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
			return 1, err.Error()
		}
		if err := command.Start(); err != nil {
			_ = windows.CloseHandle(watched)
			return 126, err.Error()
		}
		done := make(chan struct{})
		defer close(done)
		go endWithAsker(watched, command.Process, done)
		err := command.Wait()
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			return exit.ExitCode(), ""
		}
		if err != nil {
			return 1, err.Error()
		}
		return 0, ""
	}
}
