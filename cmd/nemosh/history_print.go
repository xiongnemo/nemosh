package main

import "github.com/xiongnemo/nemosh/internal/shell/runtime"

// init lends the runtime this package's history expansion, for `history -p`, which
// expands its words against the list the way `!!` is expanded at a prompt. The runtime
// has the builtin and the list; the expansion lives here, beside the loops that run it.
func init() {
	runtime.SetHistoryExpander(func(line string, entries []string) (string, error) {
		expanded, _, err := expandHistory(line, entries)
		return expanded, err
	})
}
