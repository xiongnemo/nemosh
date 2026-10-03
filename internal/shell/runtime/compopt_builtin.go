package runtime

import (
	"fmt"
	"slices"
	"strings"
)

// compopt is compopt_builtin: the -o options of the named specifications, or of the one the
// running completion uses, turned on with -o and off with +o, and printed when neither is given.
func (r Runtime) compopt(args []string) int {
	on, off, target, index, ok := r.parseCompopt(args)
	if !ok {
		return 2
	}
	names := args[index:]
	if target != "" {
		names = []string{target}
	}
	if len(names) == 0 {
		running := r.completions.running
		if running == nil {
			fmt.Fprintln(r.streams.Stderr, r.diagnosticPrefix()+"compopt: not currently executing completion function")
			return 1
		}
		if len(on)+len(off) == 0 {
			fmt.Fprintln(r.streams.Stdout, compoptText(*running.spec, running.name))
		}
		setCompletionOptions(running.spec, on, off)
		return 0
	}
	status := 0
	for _, name := range names {
		spec, found := r.completions.specs[name]
		switch {
		case !found:
			fmt.Fprintf(r.streams.Stderr, "%scompopt: %s: no completion specification\n", r.diagnosticPrefix(), name)
			status = 1
		case len(on)+len(off) == 0:
			fmt.Fprintln(r.streams.Stdout, compoptText(spec, name))
		default:
			setCompletionOptions(&spec, on, off)
			r.completions.specs[name] = spec
		}
	}
	return status
}

// parseCompopt reads compopt's options, as internal_getopt reads "+o:DEI": an option name
// after -o to turn on and after +o to turn off.
func (r Runtime) parseCompopt(args []string) (on, off []string, target string, index int, ok bool) {
	for ; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			return on, off, target, index + 1, true
		}
		if len(arg) < 2 || arg[0] != '-' && arg[0] != '+' {
			break
		}
		for at := 1; at < len(arg); at++ {
			if next := completionTarget(arg[at : at+1]); next != "" {
				target = next
				continue
			}
			if arg[at] != 'o' {
				r.completionUsage("compopt", fmt.Sprintf("%c%c: invalid option", arg[0], arg[at]))
				return nil, nil, "", index, false
			}
			value := arg[at+1:]
			if value == "" {
				if index+1 == len(args) {
					r.completionUsage("compopt", fmt.Sprintf("%co: option requires an argument", arg[0]))
					return nil, nil, "", index, false
				}
				index++
				value = args[index]
			}
			if !slices.Contains(compgenOptionNames, value) {
				fmt.Fprintf(r.streams.Stderr, "%scompopt: %s: invalid option name\n", r.diagnosticPrefix(), value)
				return nil, nil, "", index, false
			}
			if arg[0] == '-' {
				on = append(on, value)
			} else {
				off = append(off, value)
			}
			break
		}
	}
	return on, off, target, index, true
}

func setCompletionOptions(spec *compgenSpec, on, off []string) {
	for _, name := range on {
		spec.options[name] = true
	}
	for _, name := range off {
		delete(spec.options, name)
	}
}

// compoptText is a specification's options as print_compopts writes them: every one, -o when it
// is on and +o when it is off.
func compoptText(spec compgenSpec, name string) string {
	var text strings.Builder
	text.WriteString("compopt ")
	for _, option := range compgenOptionNames {
		if spec.options[option] {
			text.WriteString("-o " + option + " ")
		} else {
			text.WriteString("+o " + option + " ")
		}
	}
	return text.String() + completionNameText(name)
}
