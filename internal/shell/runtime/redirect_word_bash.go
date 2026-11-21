package runtime

import (
	"context"
	"fmt"
	"strings"
)

// bashRedirectPath is a redirection's word as bash expands it outside its POSIX mode, as an
// argument is: braces, field splitting and pathname expansion too, a pattern that matches
// nothing staying as written. It has to come to one word: none, or more than one, is bash's
// `WORD: ambiguous redirect`, the word as written, and the redirection fails, status 1, so the
// command does not run. `> $f` with f='a b', `> a-{x,y}`, `> foo-*` over two files and `> $e`
// with e empty are each ambiguous to bash, where busybox makes one path of the word, as
// expandRedirectOperations does under the other names, and under `set -o posix`, where bash
// neither splits nor globs the word. ok is false when the redirection failed, said already.
//
// A colon in IFS splits nothing here. A path on Windows has one after its drive, `D:/a/x`, where
// MSYS's, `/d/a/x`, has none, so Git's bash never meets this: `IFS=:` and then `read a < $tmp`,
// as Oils' read case has it, reads the file there, and would be an ambiguous redirect here.
func (r Runtime) bashRedirectPath(ctx context.Context, item word, savedStatus int) (path string, ok bool) {
	words := []word{item}
	if r.options.braceExpand && mayHoldBrace(item) {
		words = r.braceWords(item)
	}
	var fields []string
	for _, braced := range words {
		build := r.newFieldBuilder()
		build.separators = strings.ReplaceAll(build.separators, ":", "")
		expanded, patterns := r.buildWordFields(ctx, braced, savedStatus, build)
		fields = append(fields, r.globFields(expanded, patterns)...)
	}
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
