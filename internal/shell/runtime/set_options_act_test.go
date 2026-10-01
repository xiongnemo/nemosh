package runtime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntime_refusesToTruncate_whenNoClobberIsOn(t *testing.T) {
	// When
	status, stdout, stderr, dir := runCdScript(t,
		"printf 'kept\\n' > f.txt\nset -C\necho new > f.txt\necho [$?]\ncat f.txt\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
	}
	if stdout != "[1]\nkept\n" {
		t.Fatalf("stdout = %q, want the write refused and the file intact", stdout)
	}
	// busybox's words for the refusal, where they were bash's `cannot overwrite existing file`.
	if !strings.Contains(stderr, "cannot create f.txt: File exists") {
		t.Fatalf("stderr = %q, want a clobber diagnostic", stderr)
	}
	if content, err := os.ReadFile(filepath.Join(dir, "f.txt")); err != nil || string(content) != "kept\n" {
		t.Fatalf("file = %q (err %v), want it untouched", content, err)
	}
}

func TestRuntime_truncatesAnyway_whenTheOperatorIsClobber(t *testing.T) {
	// `>|` exists for exactly this: overriding noclobber.
	// When
	status, stdout, _, _ := runCdScript(t,
		"printf 'old\\n' > f.txt\nset -C\necho new >| f.txt\ncat f.txt\n")

	// Then
	if status != 0 || stdout != "new\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "new\n")
	}
}

func TestRuntime_appendsAndReadWritesUnderNoClobber(t *testing.T) {
	// Neither `>>` nor `<>` truncates, so neither is what -C guards against.
	// When
	status, stdout, stderr, _ := runCdScript(t,
		"printf 'a\\n' > f.txt\nset -C\necho b >> f.txt\ncat f.txt\n")

	// Then
	if status != 0 || stdout != "a\nb\n" {
		t.Fatalf("status = %d, stdout = %q, stderr = %q, want 0 and %q", status, stdout, stderr, "a\nb\n")
	}
}

func TestRuntime_opensForReadAndWriteWithoutTruncating(t *testing.T) {
	// `<>` defaults to descriptor 0, like every other `<` form, so writing
	// through it means naming 1.
	// When
	status, stdout, stderr, dir := runCdScript(t, "printf 'keep\\n' > f.txt\necho x 1<> f.txt\ncat f.txt\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
	}
	content, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// `<>` writes over the front and leaves the rest, because it does not
	// truncate: "keep\n" is five bytes and "x\n" replaces the first two.
	if string(content) != "x\nep\n" {
		t.Fatalf("file = %q, want the tail to survive; stdout was %q", content, stdout)
	}
}

func TestRuntime_tracesEachCommand_whenXtraceIsOn(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "set -x\necho one\necho 'two words'\n")

	// Then
	if status != 0 || stdout != "one\ntwo words\n" {
		t.Fatalf("status = %d, stdout = %q, want the commands to still run", status, stdout)
	}
	if !strings.Contains(stderr, "+ echo one") {
		t.Fatalf("stderr = %q, want a trace of the first command", stderr)
	}
	if !strings.Contains(stderr, "+ echo 'two words'") {
		t.Fatalf("stderr = %q, want the spaced argument quoted in the trace", stderr)
	}
}

// An assignment-only command is traced too, with only its values quoted. It was not traced
// at all. The transcript is busybox-w32's, measured.
func TestRuntime_tracesAssignments_whenXtraceIsOn(t *testing.T) {
	// When
	_, _, stderr := runSetScript(t, "set -x\nx=1\ny='a b' z=\nw=\"p q\" echo hi\n")

	// Then
	want := "+ x=1\n+ y='a b' z=\n+ w='p q' echo hi\n"
	if !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

// A PS4 whose expansion runs a command does not trace that command -- which would expand
// PS4 again -- and does not change the status an assignment reports.
func TestRuntime_expandsPS4WithoutTracingItself(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "PS4='+$(echo in) '\nset -x\nx=$(exit 3)\necho \"st=$?\"\n")

	// Then
	if status != 0 || stdout != "st=3\n" {
		t.Fatalf("status = %d, stdout = %q, want the assignment's own status", status, stdout)
	}
	if strings.Count(stderr, "+in ") != 3 || strings.Contains(stderr, "+in echo in") {
		t.Fatalf("stderr = %q, want each command traced once and PS4's own command not at all", stderr)
	}
}

func TestRuntime_usesPS4AsTheTracePrefix(t *testing.T) {
	// When
	_, _, stderr := runSetScript(t, "PS4='TRACE: '\nset -x\necho one\n")

	// Then
	if !strings.Contains(stderr, "TRACE: echo one") {
		t.Fatalf("stderr = %q, want PS4 used as the prefix", stderr)
	}
}

// With PS4 unset a trace has no prefix at all, as in both references; it was the default again.
func TestRuntime_tracesWithNoPrefixOncePS4IsUnset(t *testing.T) {
	_, _, stderr := runSetScript(t, "set -x\necho 1\nunset PS4\necho 2\n")
	if want := "+ echo 1\n+ unset PS4\necho 2\n"; stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

// -v needs input that is still unread when the option is set, to echo it as it is read, and
// a script here is parsed in full before any of it runs. Half-working would be the same lie
// as storing the flag and reporting it through `$-`. -n, which only has to run nothing
// more, acts; see set_noexec_test.go.
func TestRuntime_refusesTheOptionsThatNeedUnreadInput(t *testing.T) {
	for _, testCase := range []struct{ option, fragment string }{
		{option: "-v", fragment: "read one by one to be echoed"},
	} {
		t.Run(testCase.option, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, "set "+testCase.option+"\necho ran\n")

			// Then
			if status != 0 || stdout != "ran\n" {
				t.Fatalf("status = %d, stdout = %q, want the refusal not to stop the script", status, stdout)
			}
			if !strings.Contains(stderr, "not implemented") || !strings.Contains(stderr, testCase.fragment) {
				t.Fatalf("stderr = %q, want a not-implemented diagnostic saying why", stderr)
			}
		})
	}
}

func TestRuntime_exportsEveryAssignment_whenAllExportIsOn(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "set -a\nmarker=value\nenv | grep '^marker='\n")

	// Then
	if status != 0 || !strings.Contains(stdout, "marker=value") {
		t.Fatalf("status = %d, stdout = %q, want the assignment exported", status, stdout)
	}
}

func TestRuntime_leavesAnAssignmentUnexported_whenAllExportIsOff(t *testing.T) {
	// When
	_, stdout, _ := runSetScript(t, "unmarked=value\nenv | grep '^unmarked=' || echo absent\n")

	// Then
	if !strings.Contains(stdout, "absent") {
		t.Fatalf("stdout = %q, want the assignment to stay out of the environment", stdout)
	}
}

// ignoreeof is -I, as in busybox, and `set -o` names the options busybox-w32 has that this
// shell had no name for. monitor is named and refused: asking for it was "illegal option",
// which says the option does not exist.
func TestRuntime_namesBusyboxOptions(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "set -I\necho \"[$-]\"\nset -o\n")

	// Then
	if status != 0 || !strings.HasPrefix(stdout, "[I]\n") {
		t.Fatalf("status = %d, stdout = %q, want I in $-", status, stdout)
	}
	for _, name := range []string{"ignoreeof", "monitor", "nohiddenglob", "nohidsysglob", "vi"} {
		if !strings.Contains(stdout, name) {
			t.Errorf("set -o = %q, want it to list %s", stdout, name)
		}
	}
	for _, option := range []string{"-m", "-o monitor"} {
		status, _, stderr := runSetScript(t, "set "+option+"\n")
		if status != 2 || !strings.Contains(stderr, "not implemented") {
			t.Errorf("set %s = %d %q, want a refusal that names why", option, status, stderr)
		}
	}
}

// `set -o vi` is busybox's vi editing mode, which the session's line editor reads before each
// line. emacs is bash's other mode, and asking for either turns the other off, as bash has
// them; turning one off leaves the other as it was.
func TestRuntime_viAndEmacsAreTheEditorsTwoModes(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "set -o vi; echo $?\n[[ -o vi ]] && echo vi\n"+
		"set -o emacs\n[[ -o vi ]] || echo not vi\n[[ -o emacs ]] && echo emacs\n"+
		"set -o vi\n[[ -o emacs ]] || echo not emacs\nset +o vi\n[[ -o vi || -o emacs ]] || echo neither\n")

	// Then
	if want := "0\nvi\nnot vi\nemacs\nnot emacs\nneither\n"; status != 0 || stdout != want {
		t.Fatalf("status = %d, stdout = %q, stderr = %q, want %q", status, stdout, stderr, want)
	}
}

// `set -euo pipefail` is the first line of a strict bash script, and an `o` in the letters
// takes the next argument as its name in both references. Only a lone `-o` did here, so the
// line was `illegal option -o`, and under -e the script ended before it began.
func TestRuntime_takesAnOptionNameAfterOInALetterGroup(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "set -euo pipefail\necho \"[$-]\"\nset -o | grep -E '^(pipefail|nounset)'\nset +euo pipefail\necho \"[$-]\"\n")

	// Then
	if status != 0 || stdout != "[eu]\nnounset         on\npipefail        on\n[]\n" {
		t.Fatalf("status = %d, stdout = %q, stderr = %q", status, stdout, stderr)
	}
}
