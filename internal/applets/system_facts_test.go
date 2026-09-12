package applets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The Tier 1 applets: the small questions about the machine, the two file commands, and
// the time and process pair.
//
// Most answer a fact about the machine they run on, which a test cannot assert without
// sampling the same machine and proving nothing. So what is checked is the *shape* of the
// answer -- a number, a known architecture name, a well-formed UUID -- and the behaviour
// that is a decision rather than a fact: what an option does, and what is refused.

func runApplet(t *testing.T, name string, args []string, stdin string) (string, string, int) {
	t.Helper()
	applet, found := DefaultRegistry.Lookup(name)
	if !found {
		t.Fatalf("%s is not registered", name)
	}
	var stdout, stderr strings.Builder
	err := applet.Run(context.Background(), args, strings.NewReader(stdin), &stdout, &stderr)
	status := 0
	if err != nil {
		status = 1
		if code, carried := StatusCode(err); carried {
			status = code
		}
		switch {
		case errors.Is(err, ErrExitFalse):
			// The sentinel for "fail without saying anything", which is what -q and -s
			// options ask for. The shell prints nothing for it and neither does this.
		case hasStatusMessage(err):
			message, _ := StatusMessage(err)
			stderr.WriteString(message)
		case !isStatusOnly(err):
			stderr.WriteString(err.Error())
		}
	}
	return stdout.String(), stderr.String(), status
}

func TestNproc(t *testing.T) {
	t.Parallel()
	got, stderr, status := runApplet(t, "nproc", nil, "")
	if stderr != "" || status != 0 {
		t.Fatalf("stderr %q status %d", stderr, status)
	}
	count, err := strconv.Atoi(strings.TrimSpace(got))
	if err != nil || count < 1 {
		t.Fatalf("nproc printed %q", got)
	}
	if all, _, _ := runApplet(t, "nproc", []string{"--all"}, ""); all != got {
		t.Fatalf("--all gave %q where the plain form gave %q", all, got)
	}
	// The clamp is the decision worth pinning: a huge --ignore must still leave a usable
	// number, because `make -j0` is worse than a slow build.
	if held, _, _ := runApplet(t, "nproc", []string{"--ignore=100000"}, ""); strings.TrimSpace(held) != "1" {
		t.Fatalf("--ignore=100000 gave %q, want 1", held)
	}
	// Both spellings of the value.
	spaced, _, _ := runApplet(t, "nproc", []string{"--ignore", "100000"}, "")
	if strings.TrimSpace(spaced) != "1" {
		t.Fatalf("--ignore 100000 gave %q, want 1", spaced)
	}
	for _, bad := range [][]string{{"--ignore=x"}, {"--ignore=-1"}, {"--ignore"}, {"-z"}} {
		if _, _, status := runApplet(t, "nproc", bad, ""); status == 0 {
			t.Fatalf("nproc %v was accepted", bad)
		}
	}
}

func TestArch(t *testing.T) {
	t.Parallel()
	got, stderr, status := runApplet(t, "arch", nil, "")
	if stderr != "" || status != 0 {
		t.Fatalf("stderr %q status %d", stderr, status)
	}
	// The same answer uname -m gives, which is the point of not having a second mapping.
	machine, _, _ := runApplet(t, "uname", []string{"-m"}, "")
	if got != machine {
		t.Fatalf("arch said %q where uname -m said %q", got, machine)
	}
	if runtime.GOARCH == "amd64" && strings.TrimSpace(got) != "x86_64" {
		t.Fatalf("on amd64 arch said %q, want x86_64", got)
	}
}

func TestLognameAndGroups(t *testing.T) {
	t.Parallel()
	name, stderr, status := runApplet(t, "logname", nil, "")
	if status != 0 || stderr != "" {
		t.Skipf("no login name on this machine: %q", stderr)
	}
	if strings.TrimSpace(name) == "" {
		t.Fatal("logname printed nothing and did not fail")
	}
	// Its own name is accepted; anything else cannot be looked up and is refused rather
	// than answered with this session's groups.
	if _, _, status := runApplet(t, "groups", []string{strings.TrimSpace(name)}, ""); status != 0 {
		t.Fatalf("groups %s failed", strings.TrimSpace(name))
	}
	out, _, status := runApplet(t, "groups", []string{"someone-who-is-not-here"}, "")
	if status == 0 {
		t.Fatalf("groups accepted an unknown user and said %q", out)
	}
}

func TestUUIDGen(t *testing.T) {
	t.Parallel()
	shape := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\n$`)
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		got, stderr, status := runApplet(t, "uuidgen", nil, "")
		if stderr != "" || status != 0 {
			t.Fatalf("stderr %q status %d", stderr, status)
		}
		if !shape.MatchString(got) {
			// The version and variant nibbles are in the pattern on purpose: sixteen
			// random bytes would pass a looser check and not be a v4 UUID.
			t.Fatalf("uuidgen printed %q, which is not a version 4 UUID", got)
		}
		if seen[got] {
			t.Fatalf("uuidgen repeated %q", got)
		}
		seen[got] = true
	}
}

func TestUsleep(t *testing.T) {
	t.Parallel()
	started := time.Now()
	if _, stderr, status := runApplet(t, "usleep", []string{"50000"}, ""); stderr != "" || status != 0 {
		t.Fatalf("stderr %q status %d", stderr, status)
	}
	if waited := time.Since(started); waited < 40*time.Millisecond {
		// Loose on purpose: the assertion is that it waited at all, not how precisely.
		// A tight bound here would be a test that samples the machine's scheduler.
		t.Fatalf("usleep 50000 returned after %v", waited)
	}
	for _, bad := range [][]string{{}, {"x"}, {"-5"}, {"1", "2"}} {
		if _, _, status := runApplet(t, "usleep", bad, ""); status == 0 {
			t.Fatalf("usleep %v was accepted", bad)
		}
	}
}

func TestTs(t *testing.T) {
	t.Parallel()
	got, stderr, status := runApplet(t, "ts", []string{"%Y-%m-%d"}, "one\ntwo\n")
	if stderr != "" || status != 0 {
		t.Fatalf("stderr %q status %d", stderr, status)
	}
	stamped := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} one\n\d{4}-\d{2}-\d{2} two\n$`)
	if !stamped.MatchString(got) {
		t.Fatalf("ts printed %q", got)
	}
	// -i and -s report elapsed time, so a run this fast must show zeroes rather than a
	// clock reading.
	for _, option := range []string{"-i", "-s"} {
		elapsed, _, _ := runApplet(t, "ts", []string{option}, "a\nb\n")
		if elapsed != "00:00:00 a\n00:00:00 b\n" {
			t.Fatalf("ts %s printed %q", option, elapsed)
		}
	}
	// An unknown conversion is left as written rather than swallowed, so a typo shows.
	if kept, _, _ := runApplet(t, "ts", []string{"%Q"}, "x\n"); kept != "%Q x\n" {
		t.Fatalf("ts %%Q printed %q", kept)
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	write := func(text string) {
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sizeOf := func() int64 {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		return info.Size()
	}
	for _, testcase := range []struct {
		name  string
		start string
		size  string
		want  int64
	}{
		{name: "shrink", start: "hello world", size: "5", want: 5},
		{name: "grow", start: "hi", size: "10", want: 10},
		{name: "relative growth", start: "hello", size: "+3", want: 8},
		{name: "relative shrink", start: "hello", size: "-2", want: 3},
		{name: "shrinking past the start empties", start: "hi", size: "-100", want: 0},
		{name: "a binary suffix", start: "x", size: "2K", want: 2048},
		{name: "a decimal suffix", start: "x", size: "2KB", want: 2000},
		{name: "zero", start: "hello", size: "0", want: 0},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			write(testcase.start)
			if _, stderr, status := runApplet(t, "truncate", []string{"-s", testcase.size, path}, ""); stderr != "" || status != 0 {
				t.Fatalf("stderr %q status %d", stderr, status)
			}
			if got := sizeOf(); got != testcase.want {
				t.Fatalf("-s %s gave %d bytes, want %d", testcase.size, got, testcase.want)
			}
		})
	}

	t.Run("a missing file is created", func(t *testing.T) {
		made := filepath.Join(t.TempDir(), "new.txt")
		if _, stderr, status := runApplet(t, "truncate", []string{"-s", "4", made}, ""); stderr != "" || status != 0 {
			t.Fatalf("stderr %q status %d", stderr, status)
		}
		info, err := os.Stat(made)
		if err != nil || info.Size() != 4 {
			t.Fatalf("stat: %v", err)
		}
	})

	t.Run("-c leaves a missing file alone without failing", func(t *testing.T) {
		absent := filepath.Join(t.TempDir(), "absent.txt")
		if _, stderr, status := runApplet(t, "truncate", []string{"-c", "-s", "4", absent}, ""); stderr != "" || status != 0 {
			t.Fatalf("stderr %q status %d", stderr, status)
		}
		if _, err := os.Stat(absent); !os.IsNotExist(err) {
			t.Fatal("-c created the file")
		}
	})

	t.Run("refusals", func(t *testing.T) {
		for _, args := range [][]string{
			{path},              // no -s
			{"-s", "4"},         // no operand
			{"-s", "x", path},   // not a number
			{"-s", "<4", path},  // a GNU form this does not implement
			{"-s", "%50", path}, // likewise
		} {
			if _, _, status := runApplet(t, "truncate", args, ""); status == 0 {
				t.Fatalf("truncate %v was accepted", args)
			}
		}
	})
}

func TestLinkAndUnlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := filepath.Join(dir, "a.txt")
	target := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(source, []byte("data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, status := runApplet(t, "link", []string{source, target}, ""); stderr != "" || status != 0 {
		t.Fatalf("link: stderr %q status %d", stderr, status)
	}
	// One file under two names: writing through one is visible through the other.
	if err := os.WriteFile(source, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if through, err := os.ReadFile(target); err != nil || string(through) != "changed\n" {
		t.Fatalf("the link does not share the file: %q %v", through, err)
	}
	// An existing name is refused rather than replaced.
	if _, _, status := runApplet(t, "link", []string{source, target}, ""); status == 0 {
		t.Fatal("link replaced an existing name")
	}
	if _, stderr, status := runApplet(t, "unlink", []string{target}, ""); stderr != "" || status != 0 {
		t.Fatalf("unlink: stderr %q status %d", stderr, status)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("unlink left the name behind")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal("unlink removed the other name too")
	}
	// A directory is refused: that is what rmdir is for.
	if _, _, status := runApplet(t, "unlink", []string{dir}, ""); status == 0 {
		t.Fatal("unlink removed a directory")
	}
	for _, args := range [][]string{{}, {source}, {source, target, source}} {
		if _, _, status := runApplet(t, "link", args, ""); status == 0 {
			t.Fatalf("link %v was accepted", args)
		}
	}
}

func TestPidofAndKillall(t *testing.T) {
	t.Parallel()
	// Listing processes is a Windows implementation here; away from it these two can only
	// report that they cannot answer. Skipping is what keeps this a test of pidof rather
	// than of which platform it happens to be running on -- the failure it replaces said
	// "listing processes is not implemented on this platform" on the Linux runner.
	if _, err := processesNamed([]string{"anything"}, nil); err != nil {
		t.Skipf("this platform will not list processes: %v", err)
	}
	// Nothing matching is a status and no diagnostic, which is what makes
	// `pidof x || start x` read cleanly.
	out, stderr, status := runApplet(t, "pidof", []string{"no-such-process-anywhere"}, "")
	if status != 1 || out != "" || stderr != "" {
		t.Fatalf("pidof on a missing name: out %q stderr %q status %d", out, stderr, status)
	}
	if _, _, status := runApplet(t, "pidof", nil, ""); status == 0 {
		t.Fatal("pidof with no operand was accepted")
	}

	// killall -l lists what this shell can name, in number order.
	list, _, status := runApplet(t, "killall", []string{"-l"}, "")
	if status != 0 {
		t.Fatalf("killall -l failed with %d", status)
	}
	if !strings.HasPrefix(list, "HUP INT QUIT") || !strings.Contains(list, "TERM") {
		t.Fatalf("killall -l printed %q", list)
	}

	// -q is the difference between a diagnostic and none; the status is 1 either way.
	_, loud, status := runApplet(t, "killall", []string{"no-such-process-anywhere"}, "")
	if status != 1 || !strings.Contains(loud, "no process killed") {
		t.Fatalf("killall: stderr %q status %d", loud, status)
	}
	_, quiet, status := runApplet(t, "killall", []string{"-q", "no-such-process-anywhere"}, "")
	if status != 1 || quiet != "" {
		t.Fatalf("killall -q: stderr %q status %d", quiet, status)
	}
	if _, _, status := runApplet(t, "killall", nil, ""); status == 0 {
		t.Fatal("killall with no operand was accepted")
	}
}

// TestProcessesNamedMatchesWholeNames is the rule that separates pidof from pgrep, checked
// without needing a process of a particular name to exist.
func TestProcessesNamedMatchesWholeNames(t *testing.T) {
	t.Parallel()
	// The current process is running under the test binary's name, which is the one name
	// this test can rely on being there.
	self := filepath.Base(os.Args[0])
	found, err := processesNamed([]string{self}, nil)
	if err != nil {
		t.Skipf("this platform will not list processes: %v", err)
	}
	if len(found) == 0 {
		t.Skipf("did not find this test's own process under %q", self)
	}
	// A prefix of the name must not match, which is the whole difference from pgrep.
	if len(self) > 3 {
		partial, err := processesNamed([]string{self[:len(self)-2]}, nil)
		if err == nil && len(partial) > 0 {
			t.Fatalf("a partial name %q matched %d processes", self[:len(self)-2], len(partial))
		}
	}
	// -o omits, so omitting everything found leaves nothing.
	omit := map[int]bool{}
	for _, pid := range found {
		omit[pid] = true
	}
	if left, err := processesNamed([]string{self}, omit); err == nil && len(left) != 0 {
		t.Fatalf("omitting every id still left %v", left)
	}
}

func hasStatusMessage(err error) bool {
	_, carried := StatusMessage(err)
	return carried
}

func isStatusOnly(err error) bool {
	_, carried := StatusCode(err)
	return carried
}
