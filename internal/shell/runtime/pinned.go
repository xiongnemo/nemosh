package runtime

import (
	"context"

	"github.com/xiongnemo/nemosh/internal/cancelio"
)

// whilePinned runs what Ctrl-C ends -- the line typed at the prompt, a script's commands --
// with its goroutine pinned to its thread, so that the interrupt ends a call already waiting in
// Windows as well; see cancelio.Pin. A pipeline's stages are pinned each on its own goroutine.
// The traps run after it, unpinned, so that one asking a question at the console is answered.
func whilePinned(ctx context.Context, run func()) {
	defer cancelio.Pin(ctx)()
	run()
}
