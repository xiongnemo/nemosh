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

// alias implements the POSIX `alias` builtin: no operands lists every alias, a
// bare name reports that one, and `name=value` defines one.
//
// The listing is `name='value'` with no `alias ` in front, which is the format
// POSIX XCU specifies for it and what dash, bash --posix and busybox ash all
// print. The differential runner caught the prefix on its first run.
//
// -p is bash's: every alias listed as the command that defines it, `alias name='value'`, and
// nothing else done, since bash 5.3 reads no operand after it. It and `--` are the only
// options, and busybox has neither: it reads every operand as a name, and any other word
// beginning with - is one here too.
//
// Any value is taken, as both references take it. It is read as shell text where the alias is
// used (alias_expand.go), so one that does not parse is a syntax error there. It was refused
// here unless it was a list of words, `alias x='echo one; echo two'` among them.
func (r Runtime) alias(args []string) int {
	reusable := false
	for len(args) > 0 && isAliasOption(args[0]) {
		option := args[0]
		args = args[1:]
		if option == "--" {
			break
		}
		reusable = true
	}
	if len(args) == 0 || reusable {
		for _, name := range slices.Sorted(maps.Keys(r.aliases)) {
			r.printAlias(name, reusable)
		}
		return 0
	}
	status := 0
	for _, arg := range args {
		name, value, defines := strings.Cut(arg, "=")
		if !defines {
			if _, ok := r.aliases[name]; !ok {
				// busybox's wording, with no colon after the name.
				fmt.Fprintf(r.streams.Stderr, "alias: %s not found\n", name)
				status = 1
				continue
			}
			r.printAlias(name, false)
			continue
		}
		if !isAliasName(name) {
			fmt.Fprintf(r.streams.Stderr, "alias: %s: invalid alias name\n", name)
			status = 1
			continue
		}
		r.aliases[name] = value
	}
	return status
}

// isAliasOption is -p, any number of times over, or the `--` that ends the options.
func isAliasOption(arg string) bool {
	return arg == "--" || len(arg) > 1 && arg[0] == '-' && strings.Trim(arg[1:], "p") == ""
}

// printAlias lists one alias; reusable is bash's -p form, with `--` before a name that would
// read as an option.
func (r Runtime) printAlias(name string, reusable bool) {
	prefix := ""
	switch {
	case reusable && strings.HasPrefix(name, "-"):
		prefix = "alias -- "
	case reusable:
		prefix = "alias "
	}
	fmt.Fprintf(r.streams.Stdout, "%s%s=%s\n", prefix, name, singleQuoteForReuse(r.aliases[name]))
}

// unalias removes the aliases it names, as busybox's reads its operands: options up to the
// first word that is not one, or `--`, and -a removes every alias and ends the command there,
// whatever follows it. Any other option is refused with 2, and a name that is no alias is not
// found, with 1, while the rest are still removed. It took -a only alone, and anything else as
// a name: `unalias -a b` and `unalias -- a` said -a and -- were not found.
//
// Nothing to remove is bash's usage error, where busybox answers 0 and says nothing.
func (r Runtime) unalias(args []string) int {
	for len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
		option := args[0]
		args = args[1:]
		if option == "--" {
			break
		}
		if option[1] != 'a' {
			fmt.Fprintf(r.streams.Stderr, "unalias: illegal option -%c\n", option[1])
			return 2
		}
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
			fmt.Fprintf(r.streams.Stderr, "unalias: %s not found\n", name)
			status = 1
			continue
		}
		delete(r.aliases, name)
	}
	return status
}

func endsWithBlank(value string) bool {
	return strings.HasSuffix(value, " ") || strings.HasSuffix(value, "\t")
}

// isAliasName accepts any name that could actually be typed as a command word.
//
// isVariableName was too strict: it rejected `..`, `...`, `~`, and `a-b`, which
// are among the first aliases anyone defines and which busybox ash accepts.
// busybox validates nothing at all, so it also accepts `a=b` and `a b` -- names
// no command word can ever match, because one parses as an assignment and the
// other as two words. Those are refused here with a reason rather than stored
// where they could never fire.
func isAliasName(name string) bool {
	if name == "" {
		return false
	}
	for index := 0; index < len(name); index++ {
		switch name[index] {
		case ' ', '\t', '\n', '\r', '=', '|', '&', ';', '<', '>', '(', ')', '\'', '"', '`', '\\', '$':
			return false
		}
	}
	return true
}
