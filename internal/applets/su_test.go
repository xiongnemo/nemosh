package applets

import (
	"os"
	"strings"
	"testing"
)

// What su decides before anything runs. Kept separate from the launch precisely
// so it can be read here: the command line is one flattened string by the time
// Windows sees it, and a quoting mistake in it is a mistake nobody notices until
// an argument with a space in it goes missing.
func TestPlanElevation_assemblesTheCommandLine(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		args      []string
		arguments string
		program   string
		wait      bool
		test      bool
		inPlace   bool
		console   consoleHandover
	}{
		{
			// A shell to work in, which is what su with no arguments means.
			// Without -i a nemosh given no script reads stdin, and the elevated
			// process has no stdin worth reading.
			name: "a bare su asks for an interactive shell",
			args: nil, arguments: "-i", program: self,
		},
		{
			name: "root is the one name accepted, and is consumed",
			args: []string{"root"}, arguments: "-i", program: self,
		},
		{
			name: "-c passes the command through, quoted",
			args: []string{"-c", "echo hello world"}, arguments: `-c "echo hello world"`, program: self,
		},
		{
			name: "-W waits", args: []string{"-W"}, arguments: "-i", program: self, wait: true,
		},
		{
			// A test that does not wait observes nothing, so -t implies -W.
			// busybox ties them the same way.
			name: "-t implies -W", args: []string{"-t"}, arguments: "-i", program: self, wait: true, test: true,
		},
		{
			// -N leads, because cmd/nemosh reads -c by position. Only the child
			// can honour it: the console it holds open is its own.
			name: "-N is passed to the shell that owns the console",
			args: []string{"-N"}, arguments: "-N -i", program: self,
		},
		{
			name: "-N leads the command too",
			args: []string{"-N", "-c", "ls"}, arguments: "-N -c ls", program: self,
		},
		{
			// The window busybox always gives its shell is what this replaces.
			// The elevated shell is told which console to join, and waiting stops
			// being optional -- two shells reading one keyboard is the failure it
			// prevents.
			name: "with a console to join, the shell runs in it",
			args: nil, console: joinableConsole{pid: 4242},
			arguments: "--attach-console 4242 -i", program: self, wait: true, inPlace: true,
		},
		{
			name: "and the command runs in it too",
			args: []string{"-c", "ls"}, console: joinableConsole{pid: 4242},
			arguments: "--attach-console 4242 -c ls", program: self, wait: true, inPlace: true,
		},
		{
			// -N holds a console open so its output is not lost when it closes.
			// Nothing closes here, so asking for it would only add a keypress
			// between the command and the prompt.
			name: "-N is dropped when nothing is going to close",
			args: []string{"-N"}, console: joinableConsole{pid: 4242},
			arguments: "--attach-console 4242 -i", program: self, wait: true, inPlace: true,
		},
		{
			// A foreign program has no option meaning "join this console", so it
			// keeps the window it would always have had.
			name: "a named shell keeps its own window",
			args: []string{"-s", "/bin/sh", "-c", "id"}, console: joinableConsole{pid: 4242},
			arguments: "-c id", program: "/bin/sh",
		},
		{
			name:      "operands follow the command",
			args:      []string{"-c", "echo $1", "root", "one", "two"},
			arguments: `-c "echo $1" one two`, program: self,
		},
		{
			// busybox special-cases cmd.exe by basename, because /c is what it
			// takes where every other shell takes -c (suw32.c:118-120).
			//
			// Spelled with backslashes deliberately, and asserted on every
			// platform: the first version asked path/filepath for the basename,
			// which answers for the host, so on Linux the whole path came back
			// as the name, cmd.exe went unnoticed, and it was handed a -c it
			// does not take. The path is a Windows path wherever this compiles.
			name:      "a foreign cmd.exe takes /c",
			args:      []string{"-s", `C:\Windows\System32\cmd.exe`, "-c", "dir"},
			arguments: `/c dir`, program: `C:\Windows\System32\cmd.exe`,
		},
		{
			name:      "and so does a bare cmd, in any case",
			args:      []string{"-s", "CMD", "-c", "dir"},
			arguments: `/c dir`, program: "CMD",
		},
		{
			name:      "a forward-slash path is read the same way",
			args:      []string{"-s", "C:/Windows/System32/cmd.exe", "-c", "dir"},
			arguments: `/c dir`, program: "C:/Windows/System32/cmd.exe",
		},
		{
			name:      "any other foreign shell takes -c",
			args:      []string{"-s", "/bin/sh", "-c", "id"},
			arguments: `-c id`, program: "/bin/sh",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			console := test.console
			if console == nil {
				console = noConsole{}
			}

			// When
			plan, err := planElevation(test.args, nil, console)

			// Then
			if err != nil {
				t.Fatalf("planElevation(%q) = %v", test.args, err)
			}
			if plan.program != test.program {
				t.Fatalf("program = %q, want %q", plan.program, test.program)
			}
			if plan.arguments != test.arguments {
				t.Fatalf("arguments = %q, want %q", plan.arguments, test.arguments)
			}
			if plan.wait != test.wait || plan.test != test.test || plan.inPlace != test.inPlace {
				t.Fatalf("wait = %v, test = %v, inPlace = %v, want %v, %v and %v",
					plan.wait, plan.test, plan.inPlace, test.wait, test.test, test.inPlace)
			}
			if plan.directory == "" {
				t.Fatal("directory is empty: the child would start wherever ShellExecuteEx decided")
			}
		})
	}
}

// Every refusal, because each one is a case where doing the obvious thing would
// be worse than stopping.
func TestPlanElevation_refuses(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		fragment string
	}{
		{
			// Ours is not util-linux's su: no setuid, no user database, no
			// password. Accepting a name it cannot honour would be the lie.
			name: "a user that is not root", args: []string{"nemo"},
			fragment: `unknown user "nemo"`,
		},
		{
			name: "-N with a foreign shell", args: []string{"-s", "/bin/sh", "-N"},
			fragment: "-N is an option of this shell",
		},
		{
			name: "an option that does not exist", args: []string{"-Z"},
			fragment: "invalid option",
		},
		{
			name: "-c with nothing after it", args: []string{"-c"},
			fragment: "requires an argument",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, err := planElevation(test.args, nil, noConsole{})

			// Then
			if err == nil || !strings.Contains(err.Error(), test.fragment) {
				t.Fatalf("planElevation(%q) = %v, want an error containing %q", test.args, err, test.fragment)
			}
		})
	}
}

// The child rebuilds its argument vector with CommandLineToArgvW, so what goes
// in has to be spelled the way that function reads it back. These are its rules,
// and they are the ones busybox's quote_arg implements too.
func TestQuoteWindowsArgument_survivesTheRoundTrip(t *testing.T) {
	tests := []struct {
		argument string
		want     string
	}{
		{argument: "plain", want: "plain"},
		{argument: "", want: `""`},
		{argument: "a b", want: `"a b"`},
		{argument: `C:\Users\nemo`, want: `C:\Users\nemo`},
		// A trailing backslash inside quotes would escape the closing quote, so
		// the run is doubled.
		{argument: `C:\Program Files\`, want: `"C:\Program Files\\"`},
		{argument: `say "hi"`, want: `"say \"hi\""`},
		{argument: `a\"b`, want: `"a\\\"b"`},
		{argument: "tab\there", want: "\"tab\there\""},
	}
	for _, test := range tests {
		t.Run(test.argument, func(t *testing.T) {
			// When
			got := quoteWindowsArgument(test.argument)

			// Then
			if got != test.want {
				t.Fatalf("quoteWindowsArgument(%q) = %q, want %q", test.argument, got, test.want)
			}
		})
	}
}

// A process with no console: every non-Windows one, and a Windows service or CI
// runner. The elevated shell then has nowhere to join and takes a window.
type noConsole struct{}

func (noConsole) usable() bool { return false }

func (noConsole) ownerProcessID() int { return 0 }

type joinableConsole struct{ pid int }

func (joinableConsole) usable() bool { return true }

func (c joinableConsole) ownerProcessID() int { return c.pid }
