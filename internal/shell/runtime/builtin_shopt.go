package runtime

import (
	"fmt"
	"slices"
	"strings"
)

// `shopt` -- the option builtin bash keeps apart from `set -o`, with bash's names and bash's
// answers, since busybox has no shopt. The names and what each does are in shopt_table.go.

// shoptLine is how shopt prints a name and its state: bash's, padded to twenty. `shopt -o` has
// its own, shellOptionLine, and the `set -o` listing busybox's, setOptionLine.
const shoptLine = "%-20s\t%s\n"

// shoptRequest is what shopt's options asked for.
type shoptRequest struct {
	set, unset, quiet, print, setO bool
}

// shoptBuiltin is `shopt [-pqsu] [-o] [name ...]`.
func (r Runtime) shoptBuiltin(args []string) int {
	request, names, err := parseShoptArgs(args)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%sshopt: %v\nshopt: usage: shopt [-pqsu] [-o] [optname ...]\n", r.diagnosticPrefix(), err)
		return 2
	}
	if request.set && request.unset {
		fmt.Fprintln(r.streams.Stderr, r.diagnosticPrefix()+"shopt: cannot set and unset shell options simultaneously")
		return 1
	}
	switch {
	case request.setO:
		return r.shoptSetOptions(request, names)
	case len(names) > 0 && (request.set || request.unset):
		return r.toggleShopts(request.set, names)
	}
	return r.listShopts(request, names)
}

func parseShoptArgs(args []string) (shoptRequest, []string, error) {
	var request shoptRequest
	index := 0
	for ; index < len(args); index++ {
		argument := args[index]
		if argument == "--" {
			index++
			break
		}
		if len(argument) < 2 || argument[0] != '-' {
			break
		}
		for _, letter := range argument[1:] {
			switch letter {
			case 's':
				request.set = true
			case 'u':
				request.unset = true
			case 'q':
				request.quiet = true
			case 'p':
				request.print = true
			case 'o':
				request.setO = true
			default:
				return request, nil, fmt.Errorf("-%c: invalid option", letter)
			}
		}
	}
	return request, args[index:], nil
}

// listShopts prints the named options, or every one: with -s only those on, with -u only
// those off. Named, the status is 1 unless all of them are on, which is bash's and makes
// `shopt name` a test the way `shopt -q name` is. -p prints each as the command that sets it
// back.
func (r Runtime) listShopts(request shoptRequest, names []string) int {
	if len(names) == 0 {
		for _, option := range shoptOptions {
			value := r.shoptValue(option)
			if request.set && !value || request.unset && value {
				continue
			}
			r.printShopt(request, option.name, value)
		}
		return 0
	}
	status := 0
	for _, name := range names {
		option, known := lookupShoptOption(name)
		if !known {
			fmt.Fprintf(r.streams.Stderr, "%sshopt: %s: invalid shell option name\n", r.diagnosticPrefix(), name)
			status = 1
			continue
		}
		value := r.shoptValue(option)
		if !value {
			status = 1
		}
		r.printShopt(request, name, value)
	}
	return status
}

func (r Runtime) printShopt(request shoptRequest, name string, value bool) {
	switch {
	case request.quiet:
	case request.print && value:
		fmt.Fprintf(r.streams.Stdout, "shopt -s %s\n", name)
	case request.print:
		fmt.Fprintf(r.streams.Stdout, "shopt -u %s\n", name)
	default:
		fmt.Fprintf(r.streams.Stdout, shoptLine, name, onOff(value))
	}
}

// toggleShopts is -s or -u with names. Each is set or refused on its own, so a name this
// shell does not have leaves the others set, and the status is 1 -- bash's arrangement.
func (r Runtime) toggleShopts(value bool, names []string) int {
	status := 0
	for _, name := range names {
		if err := r.setShopt(name, value); err != nil {
			fmt.Fprintf(r.streams.Stderr, "%sshopt: %v\n", r.diagnosticPrefix(), err)
			status = 1
			continue
		}
		// At a prompt, where an option of the interactive layer is meant to act, it says it
		// does not.
		if option, _ := lookupShoptOption(name); option.kind == shoptRecorded && r.interactive.session {
			fmt.Fprintf(r.streams.Stderr, "%sshopt: %s: recorded, and changes nothing here: %s\n", r.diagnosticPrefix(), name, option.why)
		}
	}
	return status
}

// setShopt sets one name, or says why it cannot. A startup name is accepted and keeps its
// value, which is bash's answer for both.
func (r Runtime) setShopt(name string, value bool) error {
	option, known := lookupShoptOption(name)
	switch {
	case !known:
		return fmt.Errorf("%s: invalid shell option name", name)
	case option.kind == shoptActs || option.kind == shoptRecorded:
		*option.field(r.options) = value
	case option.kind == shoptFixed && value != option.on:
		return fmt.Errorf("%s: always %s here: %s", name, onOff(option.on), option.why)
	}
	return nil
}

// shoptSetOptions is -o: the names are `set -o`'s and so is what they do. Named options are
// printed in shopt's form, and the whole listing in `set -o`'s, as bash prints them -- in
// bash's order too, by name, where `set -o` keeps busybox's.
func (r Runtime) shoptSetOptions(request shoptRequest, names []string) int {
	if len(names) == 0 {
		specs := slices.SortedFunc(slices.Values(shellOptionSpecs), func(a, b shellOptionSpec) int {
			return strings.Compare(a.name, b.name)
		})
		for _, spec := range specs {
			value := *spec.field(r.options)
			switch {
			case request.quiet, request.set && !value, request.unset && value:
			case request.print:
				fmt.Fprintf(r.streams.Stdout, "set %co %s\n", optionSign(value), spec.name)
			default:
				fmt.Fprintf(r.streams.Stdout, shellOptionLine, spec.name, onOff(value))
			}
		}
		return 0
	}
	status := 0
	for _, name := range names {
		spec, known := shellOptionSpecByName(name)
		switch {
		case !known:
			fmt.Fprintf(r.streams.Stderr, "%sshopt: %s: invalid option name\n", r.diagnosticPrefix(), name)
			status = 1
		case request.set || request.unset:
			if err := r.setOptionName(name, request.set); err != nil {
				fmt.Fprintf(r.streams.Stderr, "%sshopt: %v\n", r.diagnosticPrefix(), err)
				status = 1
			}
		default:
			value := *spec.field(r.options)
			if !value {
				status = 1
			}
			switch {
			case request.quiet:
			case request.print:
				fmt.Fprintf(r.streams.Stdout, "set %co %s\n", optionSign(value), name)
			default:
				fmt.Fprintf(r.streams.Stdout, shoptLine, name, onOff(value))
			}
		}
	}
	return status
}

func onOff(value bool) string {
	if value {
		return "on"
	}
	return "off"
}
