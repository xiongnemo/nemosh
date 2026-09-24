package runtime

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Two things a redirection's descriptor can be besides a number written into the script.
//
// A duplication's source is a word, and POSIX expands it: `exec 3>log; fd=3; echo x >&$fd`
// writes to the log in busybox and bash. It was read as a number while the script was
// parsed, so `>&$fd` was a malformed redirection. It is kept as a word when it is not a
// plain number or `-`, and read once expanded. When `>&` has no number in front and the
// word is not a descriptor, both references take it as a file for stdout and stderr --
// `&>word` -- and so does this; with a number in front it is refused, as in both.
//
// `{name}>file` is bash's: the shell picks a free descriptor from 10 up, opens it, and
// puts its number in name. `exec {fd}>&-` closes it again. busybox has no form of it.

// namedDescriptorFloor is where bash starts looking for a descriptor to give a `{name}`.
const namedDescriptorFloor = 10

// isDescriptorName reports `{name}` as it stands in front of a redirection operator.
func isDescriptorName(text string) bool {
	name, ok := strings.CutPrefix(text, "{")
	if !ok {
		return false
	}
	name, ok = strings.CutSuffix(name, "}")
	return ok && isVariableName(name)
}

// splitDescriptorName separates `{name}` from the operator after it.
func splitDescriptorName(value string) (string, string) {
	if end := strings.IndexByte(value, '}'); end > 0 && isDescriptorName(value[:end+1]) {
		return value[1:end], value[end+1:]
	}
	return "", value
}

// resolveDuplication reads a duplication's source once its word is expanded.
func (r Runtime) resolveDuplication(ctx context.Context, operation redirectOperation, savedStatus int) (redirectOperation, error) {
	fields := r.expandCommandWord(ctx, operation.operand, savedStatus)
	if len(fields) != 1 {
		return operation, errAmbiguousRedirect
	}
	resolved, _, err := parseDupRedirect(operation.target, fields[0], fields[0])
	if err != nil && operation.bothStreams {
		// Not a descriptor, and `>&` had no number in front: a file for both streams.
		return redirectOperation{kind: redirectOutput, target: 1, bothStreams: true, path: fields[0]}, nil
	}
	if err != nil {
		return operation, fmt.Errorf("%s: bad file descriptor", fields[0])
	}
	resolved.name = operation.name
	return resolved, nil
}

// namedTarget is the descriptor a `{name}` redirection works on: for a close, the one name
// holds; otherwise the lowest free one from 10, which name is then set to.
func (r Runtime) namedTarget(table *fdTable, operation redirectOperation) (int, error) {
	if operation.kind == redirectClose {
		fd, err := strconv.Atoi(r.vars[operation.name])
		if err != nil {
			return 0, fmt.Errorf("%s: not a descriptor number", operation.name)
		}
		return fd, nil
	}
	fd := table.lowestFree(namedDescriptorFloor)
	if fd < 0 {
		return 0, fmt.Errorf("no descriptor free from %d to %d", namedDescriptorFloor, maxDescriptor)
	}
	if status := r.assignVar(operation.name, strconv.Itoa(fd)); status != 0 {
		return 0, fmt.Errorf("%s: cannot be set", operation.name)
	}
	return fd, nil
}

// lowestFree is the lowest descriptor from floor up that nothing holds, or -1.
func (t *fdTable) lowestFree(floor int) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	for fd := floor; fd <= maxDescriptor; fd++ {
		if entry, held := t.entries[fd]; !held || entry == nil {
			return fd
		}
	}
	return -1
}
