package runtime

import (
	"context"
	"fmt"
	"strconv"
)

// waitOptions are wait's: -n, which both references have (wait_next.go), and bash's -p NAME,
// which names a variable to hold the id of the job the status is for -- its pid, or the %N
// that `$!` is for a goroutine job. -p was "not a pid or valid job spec", status 2, and a
// pool that starts a job each time one ends has to know which one did.
type waitOptions struct {
	next     bool
	variable string
}

// parseWaitOptions reads wait's options, and answers the status to return instead when
// they are wrong, with bash's words.
func (r Runtime) parseWaitOptions(args []string) (waitOptions, []string, int) {
	var options waitOptions
	for len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
		option := args[0]
		args = args[1:]
		if option == "--" {
			break
		}
		for index := 1; index < len(option); index++ {
			letter := option[index]
			if letter == 'n' {
				options.next = true
				continue
			}
			if letter != 'p' {
				fmt.Fprintf(r.streams.Stderr, "wait: -%c: invalid option\n", letter)
				return options, nil, 2
			}
			// -p's name is the rest of the word, or the next one.
			name := option[index+1:]
			if name == "" {
				if len(args) == 0 {
					fmt.Fprintln(r.streams.Stderr, "wait: -p: option requires an argument")
					return options, nil, 2
				}
				name, args = args[0], args[1:]
			}
			if !isVariableName(name) {
				fmt.Fprintf(r.streams.Stderr, "wait: `%s': not a valid identifier\n", name)
				return options, nil, 1
			}
			options.variable = name
			break
		}
	}
	return options, args, 0
}

// startWaitVariable unsets -p's variable before the wait, as bash does: a wait that does not
// answer for one job leaves nothing in it.
func (r Runtime) startWaitVariable(ctx context.Context, options waitOptions) {
	if options.variable != "" {
		r.unset(ctx, []string{"--", options.variable})
	}
}

// noteWaited puts the id of the job a wait answered for in -p's variable.
func (r Runtime) noteWaited(options waitOptions, record *jobRecord) {
	if options.variable != "" {
		r.assignVar(options.variable, record.identifier())
	}
}

// identifier is the job as `$!` names it: its pid when it is a process, and %N otherwise.
func (record *jobRecord) identifier() string {
	if record.pid != 0 {
		return strconv.Itoa(record.pid)
	}
	return "%" + strconv.FormatUint(uint64(record.id), 10)
}
