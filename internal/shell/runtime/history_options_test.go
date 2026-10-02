package runtime_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// history takes bash's options: busybox's takes none, and only -c was taken here. Every
// expectation is bash 5.3's, measured, in this build's own row format and without bash's
// `line N:` prefix.
func TestHistory_takesBashsOptions(t *testing.T) {
	load := `history -s x1; history -s x2; history -s x3; history -s x4` + "\n"
	tests := []struct {
		name     string
		script   string
		want     string
		fragment string
	}{
		{name: "-s adds its words as one entry", script: "history -s echo a; history -s b\nhistory\n", want: "1  echo a\n2  b\n"},
		{name: "-d takes one out", script: load + "history -d 2; history\n", want: "1  x1\n2  x3\n3  x4\n"},
		{name: "-d counts back from the end", script: load + "history -d -1; history\n", want: "1  x1\n2  x2\n3  x3\n"},
		{name: "-d takes a range", script: load + "history -d 2-3; history\n", want: "1  x1\n2  x4\n"},
		{name: "-d takes a range counted back", script: load + "history -d -3--2; history\n", want: "1  x1\n2  x4\n"},
		{
			name: "-d past the end", script: load + "history -d 9; echo s=$?\n", want: "s=1\n",
			fragment: "history: 9: history position out of range",
		},
		{name: "-d 0", script: load + "history -d 0; echo s=$?\n", want: "s=1\n", fragment: "history: 0: history position out of range"},
		{name: "-d of a word", script: "history -d abc; echo s=$?\n", want: "s=1\n", fragment: "history: abc: invalid number"},
		{name: "-d with nothing", script: "history -d; echo s=$?\n", want: "s=2\n", fragment: "history: -d: option requires an argument\nhistory: usage:"},
		{name: "a count", script: load + "history 2\n", want: "3  x3\n4  x4\n"},
		{name: "a count that is a word", script: "history abc; echo s=$?\n", want: "s=2\n", fragment: "history: abc: numeric argument required"},
		{name: "two counts", script: "history 1 2; echo s=$?\n", want: "s=2\n", fragment: "history: too many arguments"},
		{name: "an option it does not take", script: "history -x; echo s=$?\n", want: "s=2\n", fragment: "history: -x: invalid option"},
		{name: "-c with -s", script: load + "history -cs after; history\n", want: "1  after\n"},
		{name: "two of -anrw", script: "history -ar f; echo s=$?\n", want: "s=1\n", fragment: "history: cannot use more than one of -anrw"},
		{
			name: "a file option with no HISTFILE", script: "unset HISTFILE; history -w; echo s=$?\n", want: "s=1\n",
			fragment: "history: HISTFILE: parameter null or not set",
		},
		{name: "a FILE that is empty", script: "history -w ''; echo s=$?\n", want: "s=1\n", fragment: "history: empty filename"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, stdout, stderr := runSetScript(t, test.script)

			// Then
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q; stderr %q", test.script, stdout, test.want, stderr)
			}
			if !strings.Contains(stderr, test.fragment) || (test.fragment == "" && stderr != "") {
				t.Fatalf("stderr = %q, want %q", stderr, test.fragment)
			}
		})
	}
}

// -w writes the list in place of the file, -r adds the file to the list, -a writes what the
// file does not have, and -n reads what it has that the list has not read.
func TestHistory_movesTheListToAndFromAFile(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	script := strings.Join([]string{
		"cd " + dir,
		"printf 'old\\n' > w; HISTFILE=w; history -c; history -s new; history -w; cat w",
		"history -c; printf 'r1\\nr2\\n' > r; history -r r; history -r r; history",
		"history -c; history -s a1; history -s a2; history -a appended; history -s a3; history -a appended; cat appended",
		"history -c; printf 'n1\\n' > n; history -r n; printf 'n2\\nn3\\n' >> n; history -n n; history",
		"history -a fromn; cat fromn",
		"history -w " + dir + "/no/such/dir/f; echo w=$?; history -r " + dir + "/nothere; echo r=$?",
		"history -s q; history -a " + dir + "/no/such/dir/f; echo a=$?",
	}, "\n") + "\n"

	// When
	_, stdout, stderr := runSetScript(t, script)

	// Then
	want := "new\n" +
		"1  r1\n2  r2\n3  r1\n4  r2\n" +
		"a1\na2\na3\n" +
		"1  n1\n2  n2\n3  n3\n" +
		"n2\nn3\n" +
		"w=1\nr=1\n" +
		"a=1\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q; stderr %q", stdout, want, stderr)
	}
	if !strings.Contains(stderr, "/no/such/dir/f: cannot create: ") {
		t.Fatalf("stderr = %q, want -a to say it cannot create the file", stderr)
	}
}

// -s and -p put their words where the command that ran them was: at a prompt, the line
// `history -s x` is recorded before it runs, and bash takes it back out, once a line.
func TestHistory_sTakesTheLineThatRanItBackOut(t *testing.T) {
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stdout})
	rt.RecordInteractiveLine("echo before", true)
	rt.RecordInteractiveLine("history -s x; history -s y", true)

	// When
	rt.RunHistoryBuiltin([]string{"-s", "x"})
	rt.RunHistoryBuiltin([]string{"-s", "y"})

	// Then
	if got := rt.HistoryEntries(); !slices.Equal(got, []string{"echo before", "x", "y"}) {
		t.Fatalf("history = %q, want the line replaced by x and y", got)
	}
}

// -a writes only what the file has not got: a line the session wrote as it was typed is not
// written again, and one `history -s` added is.
func TestHistory_aSkipsWhatTheSessionWrote(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h")
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stdout})
	rt.RecordInteractiveLine("typed and written", true)
	rt.RecordInteractiveLine("history -s pushed", false)
	rt.RunHistoryBuiltin([]string{"-s", "pushed"})

	// When
	status := rt.RunHistoryBuiltin([]string{"-a", filepath.ToSlash(path)})

	// Then
	data, err := os.ReadFile(path)
	if status != 0 || err != nil || string(data) != "pushed\n" {
		t.Fatalf("status %d, file %q, %v; want only the pushed line", status, data, err)
	}
}
