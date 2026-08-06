package runtime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// The grammar is busybox-w32 parse_interpreter (win32/process.c:66): a 99-byte
// window, a literal #!, a newline inside that window, one interpreter token, and
// at most one option taken as a single argument.
func TestParseInterpreter_followsTheBusyboxShebangGrammar(t *testing.T) {
	dir := t.TempDir()
	longLine := "#!/bin/" + strings.Repeat("x", 120) + "\n"
	for _, testCase := range []struct {
		label   string
		name    string
		content string
		want    interpreter
		found   bool
	}{
		{
			label:   "plain shebang",
			name:    "plain",
			content: "#!/bin/sh\necho hi\n",
			want:    interpreter{path: "/bin/sh", name: "sh"},
			found:   true,
		},
		{
			label:   "one option",
			name:    "opt",
			content: "#!/bin/sh -x\necho hi\n",
			want:    interpreter{path: "/bin/sh", name: "sh", opts: "-x"},
			found:   true,
		},
		{
			label:   "surrounding whitespace is trimmed and the rest is one argument",
			name:    "spaced",
			content: "#!   /bin/sh   -x -y  \necho hi\n",
			want:    interpreter{path: "/bin/sh", name: "sh", opts: "-x -y"},
			found:   true,
		},
		{
			label:   "a CRLF shebang leaves no option behind",
			name:    "crlf",
			content: "#!/bin/sh\r\necho hi\r\n",
			want:    interpreter{path: "/bin/sh", name: "sh"},
			found:   true,
		},
		{
			label:   "a CRLF option loses its carriage return",
			name:    "crlfopt",
			content: "#!/bin/sh -x\r\necho hi\r\n",
			want:    interpreter{path: "/bin/sh", name: "sh", opts: "-x"},
			found:   true,
		},
		{
			label:   "a shebang wins over the .sh fallback",
			name:    "override.sh",
			content: "#!/bin/cat\nhello\n",
			want:    interpreter{path: "/bin/cat", name: "cat"},
			found:   true,
		},
		{
			label:   "a .sh file with no shebang defaults to the shell",
			name:    "bare.sh",
			content: "echo hi\n",
			want:    interpreter{path: "/bin/sh", name: "sh"},
			found:   true,
		},
		{
			label:   "the .sh test ignores case",
			name:    "bare.SH",
			content: "echo hi\n",
			want:    interpreter{path: "/bin/sh", name: "sh"},
			found:   true,
		},
		{
			label:   "an over-long first line still reaches the .sh fallback",
			name:    "long.sh",
			content: longLine,
			want:    interpreter{path: "/bin/sh", name: "sh"},
			found:   true,
		},
		{label: "no shebang and no .sh suffix", name: "plain.txt", content: "echo hi\n"},
		{label: "a shebang with no newline in the window", name: "unterminated", content: "#!/bin/sh"},
		{label: "an over-long first line", name: "long", content: longLine},
		{label: "too short to judge", name: "tiny", content: "#!/"},
		{label: "a shebang naming nothing", name: "empty", content: "#!\necho hi\n"},
		{label: "a shebang naming a directory", name: "dir", content: "#!/bin/\necho hi\n"},
		{label: "not a shebang at all", name: "hash", content: "# not a shebang\n"},
	} {
		t.Run(testCase.label, func(t *testing.T) {
			path := filepath.Join(dir, testCase.name)
			if err := os.WriteFile(path, []byte(testCase.content), 0o600); err != nil {
				t.Fatalf("write %s: %v", testCase.name, err)
			}

			got, found, err := parseInterpreter(path)

			if err != nil {
				t.Fatalf("parseInterpreter(%q): %v", testCase.name, err)
			}
			if found != testCase.found {
				t.Fatalf("parseInterpreter(%q) found = %v, want %v (got %+v)", testCase.name, found, testCase.found, got)
			}
			if found && got != testCase.want {
				t.Fatalf("parseInterpreter(%q) = %+v, want %+v", testCase.name, got, testCase.want)
			}
		})
	}
}

func TestUnixInterpreterPath_matchesTheFourDirectoriesBusyboxAccepts(t *testing.T) {
	for _, testCase := range []struct {
		path string
		want bool
	}{
		{path: "/bin/sh", want: true},
		{path: "/usr/bin/env", want: true},
		{path: "/sbin/ip", want: true},
		{path: "/usr/sbin/cron", want: true},
		{path: "/usr/local/bin/python3"},
		{path: "/bin/extra/sh"},
		{path: "sh"},
		{path: `C:\tools\sh.exe`},
	} {
		t.Run(testCase.path, func(t *testing.T) {
			if got := unixInterpreterPath(testCase.path); got != testCase.want {
				t.Fatalf("unixInterpreterPath(%q) = %v, want %v", testCase.path, got, testCase.want)
			}
		})
	}
}

// busybox mingw_spawn_interpreter (win32/process.c:301) rebuilds argv as
// [interpreter, option?, absolute script path, arguments...], dropping the
// caller's argv[0]. An interpreter under a Unix directory is answered by this
// binary: /bin/sh by re-entering the shell, an applet name by naming the applet.
func TestPlanScriptLaunch_handsTheScriptToItsInterpreter(t *testing.T) {
	dir := t.TempDir()
	const self = `C:\nemosh\nemosh.exe`
	rt := New(applets.DefaultRegistry, Streams{})

	for _, testCase := range []struct {
		label      string
		name       string
		content    string
		executable string
		args       []string
	}{
		{
			label:      "a file that is not a script is launched as it stands",
			name:       "program.txt",
			content:    "just words\n",
			executable: "program.txt",
			args:       []string{"x"},
		},
		{
			label:      "a .sh file re-enters nemosh as a child process",
			name:       "bare.sh",
			content:    "echo hi\n",
			executable: self,
			args:       []string{"bare.sh", "x"},
		},
		{
			label:      "an interpreter named /bin/sh is this shell",
			name:       "explicit",
			content:    "#!/bin/sh\necho hi\n",
			executable: self,
			args:       []string{"explicit", "x"},
		},
		{
			label:      "an interpreter under a Unix directory names an applet",
			name:       "show",
			content:    "#!/bin/cat\nhello\n",
			executable: self,
			args:       []string{"cat", "show", "x"},
		},
		{
			label:      "an interpreter option is passed ahead of the script",
			name:       "quiet",
			content:    "#!/bin/sed -n\np\n",
			executable: self,
			args:       []string{"sed", "-n", "quiet", "x"},
		},
	} {
		t.Run(testCase.label, func(t *testing.T) {
			path := filepath.Join(dir, testCase.name)
			if err := os.WriteFile(path, []byte(testCase.content), 0o700); err != nil {
				t.Fatalf("write %s: %v", testCase.name, err)
			}

			executable, args, err := rt.planScriptLaunch(path, []string{"x"}, self)

			if err != nil {
				t.Fatalf("planScriptLaunch(%q): %v", testCase.name, err)
			}
			wantExecutable := testCase.executable
			if wantExecutable != self {
				wantExecutable = filepath.Join(dir, wantExecutable)
			}
			wantArgs := make([]string, len(testCase.args))
			for index, arg := range testCase.args {
				wantArgs[index] = arg
				if arg == testCase.name {
					wantArgs[index] = path
				}
			}
			if executable != wantExecutable || !slices.Equal(args, wantArgs) {
				t.Fatalf("planScriptLaunch(%q) = %q %q, want %q %q", testCase.name, executable, args, wantExecutable, wantArgs)
			}
		})
	}
}

// Each interpreter that is itself a script is resolved in turn, and busybox
// gives up at the fifth (++level > 4 -> ELOOP, win32/process.c:314) so a ring of
// scripts cannot spin forever.
func TestPlanScriptLaunch_stopsAfterFourInterpreters(t *testing.T) {
	dir := t.TempDir()
	const self = `C:\nemosh\nemosh.exe`
	names := []string{"a.sh", "b.sh", "c.sh", "d.sh", "e.sh"}
	for index, name := range names {
		content := "echo hi\n"
		if index+1 < len(names) {
			content = fmt.Sprintf("#!%s\necho hi\n", names[index+1])
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o700); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	t.Setenv("PATH", dir)
	rt := New(applets.DefaultRegistry, Streams{})

	t.Run("four hops arrive at the shell", func(t *testing.T) {
		executable, args, err := rt.planScriptLaunch(filepath.Join(dir, "b.sh"), []string{"x"}, self)

		if err != nil {
			t.Fatalf("planScriptLaunch(b.sh): %v", err)
		}
		want := []string{
			filepath.Join(dir, "e.sh"),
			filepath.Join(dir, "d.sh"),
			filepath.Join(dir, "c.sh"),
			filepath.Join(dir, "b.sh"),
			"x",
		}
		if executable != self || !slices.Equal(args, want) {
			t.Fatalf("planScriptLaunch(b.sh) = %q %q, want %q %q", executable, args, self, want)
		}
	})

	t.Run("five is one too many", func(t *testing.T) {
		_, _, err := rt.planScriptLaunch(filepath.Join(dir, "a.sh"), []string{"x"}, self)

		if !errors.Is(err, errInterpreterLoop) {
			t.Fatalf("planScriptLaunch(a.sh) error = %v, want %v", err, errInterpreterLoop)
		}
	})
}

func TestPlanScriptLaunch_reportsAnInterpreterThatIsNotThere(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "script")
	if err := os.WriteFile(path, []byte("#!nemosh-no-such-interpreter\n"), 0o700); err != nil {
		t.Fatalf("write script: %v", err)
	}
	t.Setenv("PATH", dir)
	rt := New(applets.DefaultRegistry, Streams{})

	_, _, err := rt.planScriptLaunch(path, nil, `C:\nemosh\nemosh.exe`)

	if !errors.Is(err, errExternalNotFound) {
		t.Fatalf("planScriptLaunch error = %v, want %v", err, errExternalNotFound)
	}
	if !strings.Contains(err.Error(), "nemosh-no-such-interpreter") {
		t.Fatalf("planScriptLaunch error = %q, want it to name the interpreter", err)
	}
}
