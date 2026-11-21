package runtime

import (
	"errors"
	"fmt"
)

// readUsage is bash 5.3's usage line for read.
const readUsage = "read: usage: read [-Eers] [-a array] [-d delim] [-i text] [-n nchars] [-N nchars] [-p prompt] [-t timeout] [-u fd] [name ...]"

// refuseReadOptions says why read's operands cannot be taken, and answers its status. Bash mode
// says it as bash's read does, measured against Git for Windows' bash 5.3: a value -n, -N, -u or
// -t cannot use, status 1, -u's an `invalid file descriptor specification`; an option read has
// not got, `-Y: invalid option`, or one missing its argument, with read's usage, 2; a name it
// cannot assign, said to be not a valid identifier, 1. nemosh and sh say what they said, every
// option error 2, as busybox answers.
func (r Runtime) refuseReadOptions(err error) int {
	if !r.BashMode() {
		fmt.Fprintf(r.streams.Stderr, "%sread: %v\n", r.diagnosticPrefix(), err)
		if _, name := errors.AsType[readNameError](err); name {
			return 1
		}
		return 2
	}
	switch refused := err.(type) {
	case readValueError:
		text := refused.Error()
		if refused.option == 'u' {
			text += " specification"
		}
		fmt.Fprintf(r.streams.Stderr, "%sread: %s\n", r.diagnosticPrefix(), text)
		return 1
	case readUsageError:
		text := fmt.Sprintf("-%c: invalid option", refused.option)
		if refused.missing {
			text = refused.Error()
		}
		fmt.Fprintf(r.streams.Stderr, "%sread: %s\n%s\n", r.diagnosticPrefix(), text, readUsage)
		return 2
	case readNameError:
		fmt.Fprintf(r.streams.Stderr, "%sread: `%s': not a valid identifier\n", r.diagnosticPrefix(), refused.name)
		return 1
	}
	fmt.Fprintf(r.streams.Stderr, "%sread: %v\n", r.diagnosticPrefix(), err)
	return 2
}

// refuseReadDescriptor says that -u's descriptor cannot be read, status 1: in bash mode, for one
// not open, bash's `99: invalid file descriptor: Bad file descriptor`.
func (r Runtime) refuseReadDescriptor(descriptor int, err error) int {
	if r.BashMode() && (errors.Is(err, errDescriptorAbsent) || errors.Is(err, errDescriptorClosed)) {
		fmt.Fprintf(r.streams.Stderr, "%sread: %d: invalid file descriptor: Bad file descriptor\n", r.diagnosticPrefix(), descriptor)
		return 1
	}
	fmt.Fprintf(r.streams.Stderr, "%sread: %v\n", r.diagnosticPrefix(), closedFor("read", err))
	return 1
}
