package runtime

import "fmt"

func (r Runtime) wait(args []string) int {
	if len(args) == 0 {
		return 0
	}
	fmt.Fprintln(r.streams.Stderr, "wait: operands are not supported yet")
	return 2
}
