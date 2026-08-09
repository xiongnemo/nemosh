package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Allocation counts are the performance baseline this project gates on, because
// they are the part of performance that is deterministic. Wall-clock time is not:
// five consecutive startups on an idle machine measured 43, 45, 42, 26 and 43
// milliseconds, and a threshold that flaps on a shared CI runner gets ignored,
// which leaves less behind than having no threshold at all.
//
// The ceilings below are not targets. Each is about twice the figure measured
// when it was written -- the measurements are in the cases themselves -- so
// ordinary refactoring moves nothing and a change of algorithm, copying a table
// per command or re-parsing inside a loop, is what trips them. When one does
// trip, the question is what started allocating, not how to raise the number.

func allocations(t *testing.T, work func()) int {
	t.Helper()
	return int(testing.AllocsPerRun(50, work))
}

func TestAllocations_parsingAScript(t *testing.T) {
	for _, test := range []struct {
		name    string
		source  string
		ceiling int
	}{
		{name: "a simple command", source: "echo hello world\n", ceiling: 140},             // measured 66
		{name: "a pipeline", source: "echo hi | cat | cat\n", ceiling: 170},                // measured 83
		{name: "a for loop", source: "for i in 1 2 3; do echo $i; done\n", ceiling: 180},   // measured 89
		{name: "a function definition", source: "f() { echo a; echo b; }\n", ceiling: 280}, // measured 135
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			count := allocations(t, func() {
				if _, err := ParseScript(test.source); err != nil {
					t.Fatalf("ParseScript(%q): %v", test.source, err)
				}
			})

			// Then
			t.Logf("%d allocations", count)
			if count > test.ceiling {
				t.Fatalf("parsing %q took %d allocations, over the %d ceiling", test.source, count, test.ceiling)
			}
		})
	}
}

// Parsing the same script twice must cost what parsing it once costs, twice.
// A parser that memoizes into shared state would be faster and wrong; one that
// accumulates state would be slower and leak. This pins the shape rather than
// the number, so it holds across toolchains where the ceilings above may not.
func TestAllocations_parsingIsProportionalToWork(t *testing.T) {
	const source = "for i in 1 2 3; do echo $i | cat; done\n"

	// When
	once := allocations(t, func() { _, _ = ParseScript(source) })
	tenTimes := allocations(t, func() {
		for range 10 {
			_, _ = ParseScript(source)
		}
	})

	// Then: ten parses cost about ten parses. The bounds are wide because the
	// point is the shape -- sublinear would mean shared state, superlinear would
	// mean accumulated state -- not the constant.
	if tenTimes < once*8 {
		t.Fatalf("one parse allocated %d and ten allocated %d; parsing appears to share state between calls", once, tenTimes)
	}
	if tenTimes > once*13 {
		t.Fatalf("one parse allocated %d and ten allocated %d; parsing appears to accumulate state", once, tenTimes)
	}
}

func TestAllocations_runningACommand(t *testing.T) {
	for _, test := range []struct {
		name    string
		script  string
		ceiling int
	}{
		{name: "a builtin", script: "true\n", ceiling: 120},                                        // measured 58
		{name: "an applet", script: "echo hello > /dev/null\n", ceiling: 240},                      // measured 110
		{name: "a pipeline of applets", script: "echo hi | cat | cat > /dev/null\n", ceiling: 700}, // measured 335
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given: one runtime for every iteration, so what is measured is
			// running the command rather than building a shell.
			var stdout, stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{
				Stdin:  strings.NewReader(""),
				Stdout: &stdout,
				Stderr: &stderr,
			})

			// When
			count := allocations(t, func() {
				if status := rt.RunScript(context.Background(), test.script); status != 0 {
					t.Fatalf("script %q exited %d, stderr = %q", test.script, status, stderr.String())
				}
			})

			// Then
			t.Logf("%d allocations", count)
			if count > test.ceiling {
				t.Fatalf("running %q took %d allocations, over the %d ceiling", test.script, count, test.ceiling)
			}
		})
	}
}

func BenchmarkParseScript(b *testing.B) {
	const source = "for i in 1 2 3; do echo $i | cat; done\n"
	b.ReportAllocs()
	for b.Loop() {
		if _, err := ParseScript(source); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRunScript_pipeline(b *testing.B) {
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	b.ReportAllocs()
	for b.Loop() {
		if status := rt.RunScript(context.Background(), "echo hi | cat | cat > /dev/null\n"); status != 0 {
			b.Fatalf("exited %d, stderr = %q", status, stderr.String())
		}
	}
}
