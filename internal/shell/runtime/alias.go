package runtime

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// maxAliasSubstitutions bounds the chain `alias a=b; alias b=c; c` can build.
// POSIX only requires that a name is not re-substituted into its own value; the
// ceiling is here so a cycle through several names cannot spin either.
const maxAliasSubstitutions = 16

var errAliasValueNotWords = fmt.Errorf("an alias value must be a list of words")

// alias implements the POSIX `alias` builtin: no operands lists every alias in
// a form that reads back in, a bare name reports that one, and `name=value`
// defines one.
func (r Runtime) alias(args []string) int {
	if len(args) == 0 {
		for _, name := range slices.Sorted(maps.Keys(r.aliases)) {
			fmt.Fprintf(r.streams.Stdout, "alias %s=%s\n", name, singleQuoteForReuse(r.aliases[name]))
		}
		return 0
	}
	status := 0
	for _, arg := range args {
		name, value, defines := strings.Cut(arg, "=")
		if !defines {
			existing, ok := r.aliases[name]
			if !ok {
				fmt.Fprintf(r.streams.Stderr, "alias: %s: not found\n", name)
				status = 1
				continue
			}
			fmt.Fprintf(r.streams.Stdout, "alias %s=%s\n", name, singleQuoteForReuse(existing))
			continue
		}
		if !isVariableName(name) {
			fmt.Fprintf(r.streams.Stderr, "alias: %s: invalid alias name\n", name)
			status = 1
			continue
		}
		if _, err := aliasWords(value); err != nil {
			fmt.Fprintf(r.streams.Stderr, "alias: %s: %v\n", name, err)
			status = 1
			continue
		}
		r.aliases[name] = value
	}
	return status
}

func (r Runtime) unalias(args []string) int {
	if len(args) == 1 && args[0] == "-a" {
		clear(r.aliases)
		return 0
	}
	if len(args) == 0 {
		fmt.Fprintln(r.streams.Stderr, "unalias: missing name")
		return 2
	}
	status := 0
	for _, name := range args {
		if _, ok := r.aliases[name]; !ok {
			fmt.Fprintf(r.streams.Stderr, "unalias: %s: not found\n", name)
			status = 1
			continue
		}
		delete(r.aliases, name)
	}
	return status
}

// substituteAliases replaces a command name that is an alias with the words its
// value stands for.
//
// POSIX has this happen during tokenization, which Nemosh cannot do: parsing
// completes before any command runs (P0.3), so no alias defined by the script
// exists yet when its own lines are parsed. Substituting here instead means an
// alias takes effect on the command that names it, including one defined
// earlier in the same script -- more useful than the POSIX timing, and the one
// place the difference shows is that an alias cannot introduce syntax.
//
// A name is not substituted into its own expansion, which is the rule that lets
// `alias ls='ls --color'` mean what it looks like. A value ending in a blank
// makes the word after it eligible too, which is what `alias sudo='sudo '` is
// for.
func (r Runtime) substituteAliases(args []string) []string {
	if len(r.aliases) == 0 || len(args) == 0 {
		return args
	}
	seen := make(map[string]bool, maxAliasSubstitutions)
	index := 0
	nextEligible := -1
	for len(seen) < maxAliasSubstitutions {
		value, defined := r.aliases[args[index]]
		if defined && !seen[args[index]] {
			words, err := aliasWords(value)
			if err != nil || len(words) == 0 {
				return args
			}
			seen[args[index]] = true
			if endsWithBlank(value) {
				nextEligible = index + len(words)
			}
			expanded := make([]string, 0, len(args)+len(words)-1)
			expanded = append(expanded, args[:index]...)
			expanded = append(expanded, words...)
			args = append(expanded, args[index+1:]...)
			// The replacement is re-examined at the same position, which is how
			// `alias a=b; alias b=ls` reaches ls. `seen` is what stops
			// `alias ls='ls --color'` from looping on itself.
			continue
		}
		if nextEligible < 0 || nextEligible >= len(args) {
			return args
		}
		index, nextEligible = nextEligible, -1
	}
	return args
}

func endsWithBlank(value string) bool {
	return strings.HasSuffix(value, " ") || strings.HasSuffix(value, "\t")
}

// aliasWords tokenizes an alias value into the words it stands for, and refuses
// one that carries an operator.
//
// Refusing is the honest answer rather than a silent limitation: substituting
// here rather than during tokenization means an alias contributes words to a
// command that has already been parsed, so `alias c='a | b'` could not build the
// pipeline it promises. Saying so at definition time is better than accepting it
// and running something else.
func aliasWords(value string) ([]string, error) {
	tokens, err := scanShellTokens(value)
	if err != nil {
		return nil, err
	}
	words := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token.kind != tokenWord {
			return nil, errAliasValueNotWords
		}
		words = append(words, token.value)
	}
	return words, nil
}
