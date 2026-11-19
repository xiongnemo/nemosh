package applets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A differential sweep of diff against busybox-w32, which prints what it prints with no
// expectation of its own: generated pairs of files, from a fixed seed, each diffed under one
// set of options, and the two outputs and statuses compared byte for byte. The lines come
// from a small vocabulary, so most of them repeat, which is where two diffs of the same pair
// most often choose different edit scripts; a few pairs are long enough that stone gives up on
// a class, which -d does not. It skips where busybox-w32 is not installed, as on CI, where the
// literal cases in diff_busybox_test.go hold instead.
func TestDiffDifferentialSweep(t *testing.T) {
	if !busyboxIsTheReference() {
		t.Skip("busybox-w32 is not installed")
	}
	busybox, _ := exec.LookPath("busybox")
	optionSets := [][]string{nil, {"-i"}, {"-w"}, {"-b"}, {"-B"}, {"-d"}, {"-U0"}, {"-U5"}, {"-bB"}, {"-iw"}}
	random := rand.New(rand.NewPCG(2026, 1119))
	dir := t.TempDir()
	ctx := WithProcessView(context.Background(), hostProcessView{cwd: dir})
	compared, differed, first := 0, 0, ""
	for index := range 400 {
		lines := 3 + random.IntN(60)
		if index%40 == 39 {
			lines = 1500 + random.IntN(1500)
		}
		left := diffSweepFile(random, lines)
		right := diffSweepMutate(random, left)
		files := map[string][]string{"a": left, "b": right}
		for name, text := range files {
			body := strings.Join(text, "")
			if random.IntN(6) == 0 {
				body = strings.TrimSuffix(body, "\n")
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		options := optionSets[index%len(optionSets)]
		args := append(append([]string{}, options...), "a", "b")
		command := exec.Command(busybox, append([]string{"diff"}, args...)...)
		command.Dir = dir
		want, err := command.Output()
		wantStatus := 0
		if exit := (*exec.ExitError)(nil); errors.As(err, &exit) {
			wantStatus = exit.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		var got, errs bytes.Buffer
		runErr := newDiffApplet().Run(ctx, args, strings.NewReader(""), &got, &errs)
		gotStatus := 0
		switch {
		case errors.Is(runErr, ErrExitFalse):
			gotStatus = 1
		case runErr != nil:
			gotStatus, _ = StatusCode(runErr)
		}
		compared++
		if got.String() == string(want) && gotStatus == wantStatus {
			continue
		}
		if differed++; differed == 1 {
			a, _ := os.ReadFile(filepath.Join(dir, "a"))
			b, _ := os.ReadFile(filepath.Join(dir, "b"))
			first = fmt.Sprintf("case %d, diff %q: status %d, busybox %d\n--- a\n%q\n--- b\n%q\n--- nemosh\n%s\n--- busybox\n%s",
				index, args, gotStatus, wantStatus, a, b, got.String(), want)
		}
	}
	if differed > 0 {
		t.Fatalf("%d of %d pairs differ from busybox; the first:\n%s", differed, compared, first)
	}
	t.Logf("%d pairs agree with busybox", compared)
}

// diffSweepVocabulary is what the generated lines are made of: lines code repeats, some with
// capitals and white space that -i, -w and -b read alike, and the empty lines -B passes over.
var diffSweepVocabulary = []string{
	"}\n", "\n", "\treturn nil\n", "if err != nil {\n", "\t}\n", "a\n", "b\n", "A\n",
	"x  y\n", "x y\n", " x y \n", "\t\n", "  \n", "Return Nil\n",
}

func diffSweepFile(random *rand.Rand, count int) []string {
	lines := make([]string, count)
	for index := range lines {
		if random.IntN(3) == 0 {
			lines[index] = fmt.Sprintf("line %d\n", random.IntN(count))
		} else {
			lines[index] = diffSweepVocabulary[random.IntN(len(diffSweepVocabulary))]
		}
	}
	return lines
}

// diffSweepMutate deletes, inserts, changes and moves lines of a copy of lines.
func diffSweepMutate(random *rand.Rand, lines []string) []string {
	out := append([]string(nil), lines...)
	for range 1 + random.IntN(max(1, len(lines)/8)) {
		at := random.IntN(len(out) + 1)
		switch random.IntN(4) {
		case 0:
			if at < len(out) {
				out = append(out[:at], out[at+1:]...)
			}
		case 1:
			out = append(out[:at], append([]string{diffSweepVocabulary[random.IntN(len(diffSweepVocabulary))]}, out[at:]...)...)
		case 2:
			if at < len(out) {
				out[at] = fmt.Sprintf("changed %d\n", random.IntN(100))
			}
		default:
			if at+5 < len(out) {
				block := append([]string(nil), out[at:at+5]...)
				out = append(out[:at], out[at+5:]...)
				to := random.IntN(len(out) + 1)
				out = append(out[:to], append(block, out[to:]...)...)
			}
		}
	}
	return out
}
