package runtime

import (
	"context"
	"fmt"
	"strings"
)

// compgen is bash's: the completions of a word, one to a line, as its actions, a glob, a word
// list, a function and a command would generate them, filtered by -X and given -P and -S. It
// answers 1 when there are none. busybox has no programmable completion, so bash is the
// reference; nemosh's own completion for the line editor is its TOML specs, which this does not
// read.
//
//	compgen [-V varname] [-abcdefgjksuv] [-o option] [-A action] [-G globpat] [-W wordlist]
//	        [-F function] [-C command] [-X filterpat] [-P prefix] [-S suffix] [word]
//
// The results come in bash's order whatever the options' order: the actions the shell knows
// (aliases, builtins, functions, keywords, variables and the rest), then the ones on the
// filesystem (commands, directories, files, users), then -G, -W, -F and -C.
func (r Runtime) compgen(ctx context.Context, args []string, savedStatus int) int {
	parsed, index, ok := r.parseCompletionOptions("compgen", args, "", "oAGWFCXPSV")
	if !ok {
		return 2
	}
	// With no option there is nothing to generate, and that is no failure, as bash's
	// build_actions answers it.
	if !parsed.given {
		return 0
	}
	spec, word := parsed.spec, ""
	if index < len(args) {
		word = args[index]
	}
	if spec.function != "" {
		fmt.Fprintln(r.streams.Stderr, r.diagnosticPrefix()+"compgen: warning: -F option may not work as you expect")
	}
	if spec.command != "" {
		fmt.Fprintln(r.streams.Stderr, r.diagnosticPrefix()+"compgen: warning: -C option may not work as you expect")
	}
	matches, _, ok := r.generateCompletions(ctx, spec, completionRequest{command: "compgen", word: word, cword: -1}, savedStatus)
	if !ok {
		return 1
	}
	if len(matches) == 0 && (spec.options["default"] || spec.options["bashdefault"]) {
		matches = r.compgenPaths(word, false)
	}
	if spec.array != "" {
		r.arrays.set(spec.array, matches)
		delete(r.vars, spec.array)
	} else {
		for _, match := range matches {
			fmt.Fprintln(r.streams.Stdout, match)
		}
	}
	if len(matches) == 0 {
		return 1
	}
	return 0
}

// withPrefix keeps the candidates that begin with word, in their order.
func withPrefix(candidates []string, word string) []string {
	var kept []string
	for _, candidate := range candidates {
		if strings.HasPrefix(candidate, word) {
			kept = append(kept, candidate)
		}
	}
	return kept
}
