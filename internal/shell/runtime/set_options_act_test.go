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
	if !strings.Contains(stderr, "cannot overwrite") {
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

func TestRuntime_usesPS4AsTheTracePrefix(t *testing.T) {
	// When
	_, _, stderr := runSetScript(t, "PS4='TRACE: '\nset -x\necho one\n")

	// Then
	if !strings.Contains(stderr, "TRACE: echo one") {
		t.Fatalf("stderr = %q, want PS4 used as the prefix", stderr)
	}
}

// -n and -v both need input that is still unread when the option is set, and a
// script here is parsed in full before any of it runs. Half-working would be
// the same lie as storing the flag and reporting it through `$-`.
func TestRuntime_refusesTheOptionsThatNeedUnreadInput(t *testing.T) {
	for _, testCase := range []struct{ option, fragment string }{
		{option: "-n", fragment: "no unread input left"},
		{option: "-v", fragment: "no lines left to echo"},
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
