package runtime

import (
	"context"
	"fmt"
)

// bashRedirectPath is a redirection's word as bash expands it outside its POSIX mode, as an
// argument is: braces, field splitting and pathname expansion too, a pattern that matches
// nothing staying as written. It has to come to one word: none, or more than one, is bash's
// `WORD: ambiguous redirect`, the word as written, and the redirection fails, status 1, so the
// command does not run. `> $f` with f='a b', `> a-{x,y}`, `> foo-*` over two files and `> $e`
// with e empty are each ambiguous to bash, where busybox makes one path of the word, as
// expandRedirectOperations does under the other names, and under `set -o posix`, where bash
// neither splits nor globs the word. ok is false when the redirection failed, said already.
func (r Runtime) bashRedirectPath(ctx context.Context, item word, savedStatus int) (path string, ok bool) {
	fields := r.expandCommandWord(ctx, item, savedStatus)
	if r.expansion.shellError {
		// failglob's `no match`, said already, which abandons the line: left raised, for the
		// command to answer as any such error is answered.
		return "", true
	}
	if len(fields) != 1 {
		fmt.Fprintf(r.streams.Stderr, "%s%s: ambiguous redirect\n", r.diagnosticPrefix(), printWord(item))
		return "", false
	}
	return fields[0], true
}
