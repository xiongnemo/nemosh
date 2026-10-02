package applets

import (
	"fmt"
	"strings"
)

// findLong is the long option name stands for, whole or by a prefix, as getopt_long finds one.
// A prefix that fits several stands for the first when they take an argument alike, as
// busybox's getopt registers each with the same flag and value and getopt_long then sees one
// option; told apart, it is ambiguous. typed is the word without its dashes, for the message
// of one nobody has. Only the whole name was taken, so `--lon` was unknown where both
// references take it for --longer.
func (r getoptRequest) findLong(name, typed string) (string, error) {
	var found []string
	for _, long := range r.longOptions {
		bare := strings.TrimRight(long, ":")
		if bare == name {
			return long, nil
		}
		if name != "" && strings.HasPrefix(bare, name) {
			found = append(found, long)
		}
	}
	if len(found) == 0 {
		return "", fmt.Errorf("unknown option -- %s", typed)
	}
	for _, other := range found[1:] {
		if argumentColons(other) != argumentColons(found[0]) {
			return "", fmt.Errorf("ambiguous option -- %s", name)
		}
	}
	return found[0], nil
}

// argumentColons is how a long option says it takes an argument: none, one required, two
// optional.
func argumentColons(long string) int {
	return len(long) - len(strings.TrimRight(long, ":"))
}
