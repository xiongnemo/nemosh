package runtime

import (
	"slices"
	"sync"
)

// readProgress is what a read with -t has taken of its line so far, kept where the read that
// runs out of time can find it. Both references assign it: of a `te` that waits for its `st`,
// `read -t 1 reply` leaves reply holding te, with status 142, where it was left empty. The read
// goes on in a goroutine of its own (collectWithTimeout), so the text is taken under a lock.
type readProgress struct {
	mu      sync.Mutex
	text    []byte
	escaped []bool
}

// add records a byte the read kept. A nil progress records nothing.
func (progress *readProgress) add(char byte, escaped bool) {
	if progress == nil {
		return
	}
	progress.mu.Lock()
	progress.text, progress.escaped = append(progress.text, char), append(progress.escaped, escaped)
	progress.mu.Unlock()
}

// line is what was read before the time ran out: a line whose delimiter never came.
func (progress *readProgress) line() readLineResult {
	progress.mu.Lock()
	defer progress.mu.Unlock()
	return readLineResult{text: string(progress.text), escaped: slices.Clone(progress.escaped)}
}
