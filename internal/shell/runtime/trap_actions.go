package runtime

import "fmt"

// printTrapActions is bash 5.3's `trap -P condition...`: each named condition's action and
// nothing else, a line each, and no line for one without a trap -- the form to capture in a
// variable, where -p's is the form to eval. busybox has neither. It was taken for an action,
// so `trap -P INT` armed INT with a command named -P.
func (r Runtime) printTrapActions(conditions []string) int {
	if len(conditions) == 0 {
		fmt.Fprintln(r.streams.Stderr, "trap: -P requires at least one signal name")
		return 2
	}
	status := 0
	for _, condition := range conditions {
		name, ok := trapConditionName(condition)
		if !ok {
			fmt.Fprintf(r.streams.Stderr, "trap: %s: invalid signal specification\n", condition)
			status = 1
			continue
		}
		if action, set := r.traps[name]; set && name != "" {
			fmt.Fprintln(r.streams.Stdout, action)
		}
	}
	return status
}
