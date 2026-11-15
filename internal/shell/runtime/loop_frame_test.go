package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// What the frames are for, measured where it matters: a loop running the same applets every
// time round is handed the same frames, so its applet calls allocate nothing. Measured as
// the difference a frame per call makes -- NEMOSH_APPLET_FRAMES=fresh -- over the loop's
// two applet calls an iteration, `[` and true, so the rest of what an iteration allocates
// does not enter into it.
func TestAllocations_aLoopsAppletsAreHandedTheirFramesAgain(t *testing.T) {
	perIteration := func(t *testing.T, mode string) float64 {
		t.Setenv("NEMOSH_APPLET_FRAMES", mode)
		count := func(iterations string) float64 {
			var stdout, stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			script := "i=0; while [ $i -lt " + iterations + " ]; do true; i=$((i+1)); done\n"
			return testing.AllocsPerRun(5, func() { rt.RunScript(ctx, script) })
		}
		return (count("200") - count("100")) / 100
	}

	// When
	reused := perIteration(t, "")
	fresh := perIteration(t, "fresh")

	// Then: two frames an iteration fewer. Within half an allocation of two, since a slice
	// that grows now and then shows up as a hundredth here: 53.01 against 55.00 is two frames.
	t.Logf("%.2f allocations an iteration, %.2f with a frame per call", reused, fresh)
	// And the iteration as a whole, which was 142 before the frames, 108 after them and 36
	// on 2026-11-16, once a plain word, a word with no brace, $PIPESTATUS, a variable read in
	// arithmetic and an assignment with no tilde stopped allocating. About a fifth over.
	if reused > 44 {
		t.Fatalf("the loop allocated %.2f an iteration, over the ceiling of 44", reused)
	}
	if fresh-reused < 1.5 {
		t.Fatalf("the loop allocated %.2f an iteration against %.2f with a frame per call; its applets were not handed their frames again", reused, fresh)
	}
}
