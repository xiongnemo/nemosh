//go:build windows

package runtime

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/xiongnemo/nemosh/internal/osrand"
	"github.com/xiongnemo/nemosh/internal/sudo"
	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

// An SSH session's shell without administrator rights, and sudo to lend them back.
//
// Windows's sshd gives an administrator's session the elevated token, with no linked unelevated one,
// so everything typed over ssh ran as administrator; and a program that refuses to run elevated
// refused to run at all, as a ChatGPT or Codex client's app server does. So a nemosh that sshd
// starts elevated runs the session as a child of its own with the token drop gives
// (unelevatedToken), and stays behind, elevated, to lend that back: `sudo COMMAND` asks it over a
// named pipe (sudoLender), as sudo with NOPASSWD would, since the session's key is the password
// already given. Every session drops, `ssh host command`, scp and sftp too, which pass through the
// default shell; NEMOSH_SSH_ADMIN=keep in the user's environment keeps them elevated, there being no
// variable Windows's sshd takes from the client.

// dropNotice is said at the start of an interactive session that dropped.
const dropNotice = "nemosh: this SSH session runs without administrator rights; `sudo COMMAND` runs a command with them, and NEMOSH_SSH_ADMIN=keep in your environment keeps them"

// DropSSHSession runs this nemosh's session unelevated and lends it sudo, when sshd started it
// elevated, and answers with the session's status and true; otherwise false, and the start goes on
// as it was. One that cannot drop says why and goes on elevated.
func DropSSHSession() (int, bool) {
	if !sshSessionDrops(os.Getenv, processIsElevated) {
		return 0, false
	}
	status, err := lendElevation()
	if err != nil {
		fmt.Fprintf(os.Stderr, "nemosh: this SSH session keeps administrator rights: %v\n", err)
		return 0, false
	}
	return status, true
}

// sshSessionDrops is whether a session drops: sshd's variables are set, the token is elevated, no
// lender is above this one already, and the user has not asked to keep the rights.
func sshSessionDrops(getenv func(string) string, elevated func() bool) bool {
	if getenv("SSH_CONNECTION") == "" && getenv("SSH_CLIENT") == "" {
		return false
	}
	if getenv(sudo.KeepVariable) == "keep" || getenv(sudo.PipeVariable) != "" {
		return false
	}
	return elevated()
}

func processIsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// sshSessionIsInteractive is whether the session's shell will prompt: no -c, no script, and a
// terminal to read; the notice is said only then, since a command's stderr is the client's.
func sshSessionIsInteractive(args []string, terminal bool) bool {
	for _, arg := range args {
		if arg == "--" || !strings.HasPrefix(arg, "-") {
			return false
		}
		if !strings.HasPrefix(arg, "--") && strings.ContainsRune(arg, 'c') {
			return false
		}
	}
	return terminal
}

// lendElevation runs this command line again with the unelevated token, the pipe's name in
// PipeVariable, and lends sudo until it ends. The console's Ctrl-C is the session's to answer.
func lendElevation() (int, error) {
	self, err := os.Executable()
	if err != nil {
		return 0, err
	}
	var suffix [8]byte
	if _, err := osrand.Read(suffix[:]); err != nil {
		return 0, err
	}
	name := fmt.Sprintf(`\\.\pipe\nemosh-sudo-%d-%x`, os.Getpid(), suffix)
	lender, err := listenForSudo(name, lendElevated(self, os.Args[0]))
	if err != nil {
		return 0, err
	}
	token, err := unelevatedToken()
	if err != nil {
		return 0, err
	}
	shell := &exec.Cmd{Path: self, Args: os.Args, Env: append(os.Environ(), sudo.PipeVariable+"="+name),
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, SysProcAttr: &syscall.SysProcAttr{Token: syscall.Token(token)}}
	signal.Ignore(os.Interrupt)
	if sshSessionIsInteractive(os.Args[1:], term.IsTerminal(int(os.Stdin.Fd()))) {
		fmt.Fprintln(os.Stderr, dropNotice)
	}
	err = shell.Start()
	_ = token.Close()
	if err != nil {
		return 0, err
	}
	go lender.serve()
	err = shell.Wait()
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return exit.ExitCode(), nil
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "nemosh: %v\n", err)
		return 1, nil
	}
	return 0, nil
}
