package runtime

import (
	"fmt"
	"sort"
	"strings"
)

// What bind lists, each in bash 5.3's words: -l the functions; -p and -P what each is on; -q
// one function; -s and -S the macros; -X the commands -x bound; -v and -V the variables.

// keysBound is the keys of keymap bound so, in the order of their bytes, as readline walks its
// keymap, each in readline's notation.
func keysBound(bound map[string]KeyBinding, match func(KeyBinding) bool) []string {
	var keys []string
	for sequence, binding := range bound {
		if match(binding) {
			keys = append(keys, sequence)
		}
	}
	sort.Strings(keys)
	for index, sequence := range keys {
		keys[index] = printKeyseq([]byte(sequence))
	}
	return keys
}

func functionBound(name string) func(KeyBinding) bool {
	return func(binding KeyBinding) bool { return binding.Kind == BindFunction && binding.Text == name }
}

// quotedKeys is keys each in double quotes, a comma between.
func quotedKeys(keys []string) string {
	return `"` + strings.Join(keys, `", "`) + `"`
}

// bindListFunctions is -p, or with readable false -P: every function and the keys it is on.
func (r Runtime) bindListFunctions(keymap string, readable bool) {
	bound := r.bindings.bound(keymap)
	for _, name := range strings.Fields(readlineFunctionNames) {
		keys := keysBound(bound, functionBound(name))
		switch {
		case readable && len(keys) == 0:
			fmt.Fprintf(r.streams.Stdout, "# %s (not bound)\n", name)
		case readable:
			for _, key := range keys {
				fmt.Fprintf(r.streams.Stdout, "\"%s\": %s\n", key, name)
			}
		case len(keys) == 0:
			fmt.Fprintf(r.streams.Stdout, "%s is not bound to any keys\n", name)
		default:
			fmt.Fprintf(r.streams.Stdout, "%s can be found on %s.\n", name, quotedKeys(keys))
		}
	}
}

// bindQuery is -q: what keys a function is on, status 1 when none or no such function.
func (r Runtime) bindQuery(keymap, name string) int {
	if !readlineHas(name) {
		fmt.Fprintf(r.streams.Stderr, "%sbind: `%s': unknown function name\n", r.diagnosticPrefix(), name)
		return 1
	}
	keys := keysBound(r.bindings.bound(keymap), functionBound(name))
	if len(keys) == 0 {
		fmt.Fprintf(r.streams.Stdout, "%s is not bound to any keys.\n", name)
		return 1
	}
	fmt.Fprintf(r.streams.Stdout, "%s can be invoked via %s.\n", name, quotedKeys(keys))
	return 0
}

// bindListTexts is -s, -S and -X: what each macro types, or each -x command runs.
func (r Runtime) bindListTexts(keymap string, kind BindingKind, readable bool) {
	bound := r.bindings.bound(keymap)
	keys := make([]string, 0, len(bound))
	for sequence, binding := range bound {
		if binding.Kind == kind {
			keys = append(keys, sequence)
		}
	}
	sort.Strings(keys)
	for _, sequence := range keys {
		key, text := printKeyseq([]byte(sequence)), bound[sequence].Text
		switch {
		case kind == BindCommand:
			fmt.Fprintf(r.streams.Stdout, "\"%s\" \"%s\"\n", key, text)
		case readable:
			fmt.Fprintf(r.streams.Stdout, "\"%s\": \"%s\"\n", key, printKeyseq([]byte(text)))
		default:
			fmt.Fprintf(r.streams.Stdout, "%s outputs %s\n", key, text)
		}
	}
}

// bindListVariables is -v, or with readable false -V.
func (r Runtime) bindListVariables(keymap string, readable bool) {
	for _, line := range strings.Split(strings.TrimSpace(readlineVariableDefaults), "\n") {
		name, _, _ := strings.Cut(line, " ")
		value := r.ReadlineVariable(name)
		switch name {
		case "editing-mode":
			value = "emacs"
			if r.options.vi {
				value = "vi"
			}
		case "keymap":
			value = keymap
		}
		if readable {
			fmt.Fprintf(r.streams.Stdout, "set %s %s\n", name, value)
			continue
		}
		fmt.Fprintf(r.streams.Stdout, "%s is set to `%s'\n", name, value)
	}
}
