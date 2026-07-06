package runtime

import (
	"fmt"
	"strconv"
	"strings"
)

func (r Runtime) getopts(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(r.streams.Stderr, "getopts: expected optstring and name")
		return 2
	}
	optstring := args[0]
	name := args[1]
	index := r.getoptsIndex()
	if index > len(r.params.values) {
		return r.endGetopts(name)
	}
	arg := r.params.values[index-1]
	if !strings.HasPrefix(arg, "-") || arg == "-" || arg == "--" {
		return r.endGetopts(name)
	}
	option := arg[1:]
	if len(option) != 1 || !getoptsHasOption(optstring, option[0]) {
		return r.endGetopts(name)
	}
	if getoptsNeedsArgument(optstring, option[0]) {
		if index >= len(r.params.values) {
			return r.endGetopts(name)
		}
		if status := r.assignVar("OPTARG", r.params.values[index]); status != 0 {
			return status
		}
		index += 2
	} else {
		delete(r.vars, "OPTARG")
		index++
	}
	if status := r.assignVar(name, option); status != 0 {
		return status
	}
	return r.assignVar("OPTIND", strconv.Itoa(index))
}

func (r Runtime) getoptsIndex() int {
	index, err := strconv.Atoi(r.vars["OPTIND"])
	if err != nil || index < 1 {
		return 1
	}
	return index
}

func (r Runtime) endGetopts(name string) int {
	_ = r.assignVar(name, "?")
	return 1
}

func getoptsHasOption(optstring string, option byte) bool {
	for i := 0; i < len(optstring); i++ {
		if optstring[i] == option {
			return true
		}
	}
	return false
}

func getoptsNeedsArgument(optstring string, option byte) bool {
	for i := 0; i+1 < len(optstring); i++ {
		if optstring[i] == option {
			return optstring[i+1] == ':'
		}
	}
	return false
}
