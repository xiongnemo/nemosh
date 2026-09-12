package applets

import (
	"crypto/rand"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"strings"
)

// The small commands that answer one question about the machine or the session.
//
// They share a file because separately each would be more boilerplate than body: a
// constructor, an option check and one line of output. What they have in common is that the
// answer is a *fact*, so the only thing that can go wrong is the question.

// nproc answers how many processors are available, which is the number a parallel build
// divides its work by.
//
// `--all` is accepted and means the same thing here. On Unix the two differ -- all the
// processors, against the ones this process is allowed -- but Windows reports only the set
// the process may use, so there is no second number to give and pretending otherwise would
// be worse than saying they agree.
//
// `--ignore=N` holds some back, and the answer **never drops below 1**: `make -j0` is a
// worse outcome than a slow build, and both references clamp the same way.
func newNprocApplet() Applet {
	return simpleApplet{name: "nproc", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		ignore, err := parseNprocIgnore(args)
		if err != nil {
			return err
		}
		count := runtime.NumCPU() - ignore
		if count < 1 {
			count = 1
		}
		_, err = fmt.Fprintln(stdout, count)
		return err
	}}
}

// parseNprocIgnore reads the long options, which parseAppletOptions does not do.
//
// Both spellings of the value are accepted, `--ignore=2` and `--ignore 2`, because both
// references take both and a script has no way to know which it was given.
func parseNprocIgnore(args []string) (int, error) {
	ignore := 0
	for index := 0; index < len(args); index++ {
		argument := args[index]
		value := ""
		switch {
		case argument == "--all":
			continue
		case argument == "--ignore":
			index++
			if index >= len(args) {
				return 0, fmt.Errorf("--ignore needs a number")
			}
			value = args[index]
		case strings.HasPrefix(argument, "--ignore="):
			value = strings.TrimPrefix(argument, "--ignore=")
		default:
			if strings.HasPrefix(argument, "-") {
				return 0, fmt.Errorf("unsupported option: %s", argument)
			}
			// Not an option, so it is an operand -- and nproc takes none. Said in those
			// words rather than as an option complaint, because it is not one.
			return 0, fmt.Errorf("extra operand '%s'", argument)
		}
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			return 0, fmt.Errorf("invalid number '%s'", value)
		}
		ignore = parsed
	}
	return ignore, nil
}

// arch is `uname -m` under the name people type for it.
//
// The same mapping, from the same function, rather than a second copy: an `arch` that said
// `amd64` where `uname -m` said `x86_64` would be a bug nobody would look for.
func newArchApplet() Applet {
	return simpleApplet{name: "arch", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		if err := refuseArguments(args); err != nil {
			return err
		}
		_, err := fmt.Fprintln(stdout, unameMachine(runtime.GOARCH))
		return err
	}}
}

// logname is the account that logged in, which is deliberately **not** `whoami`.
//
// The two differ exactly where it matters. Under elevation `whoami` answers `root`, because
// it reports the identity the process is running as; `logname` still answers the account,
// because that is who logged in. Having both is only worth anything if they can disagree,
// and busybox draws the line in the same place.
func newLognameApplet() Applet {
	return simpleApplet{name: "logname", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		if err := refuseArguments(args); err != nil {
			return err
		}
		name := accountName()
		if name == "" {
			// Loud rather than a guess: a script using this to build a path wants a
			// failure, not the empty string.
			return fmt.Errorf("no login name")
		}
		_, err := fmt.Fprintln(stdout, name)
		return err
	}}
}

// groups lists the groups a user belongs to.
//
// Windows has no group in the Unix sense, so this answers the single name the identity
// model here already uses -- the one `id -gn` gives, for the reason id.go sets out. A user
// other than this one cannot be looked up, so naming one is refused rather than answered
// with this session's groups, which would be a wrong answer that looked right.
func newGroupsApplet() Applet {
	return simpleApplet{name: "groups", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		_, operands, err := parseAppletOptions(args, "", "")
		if err != nil {
			return err
		}
		if len(operands) > 1 {
			return fmt.Errorf("extra operand '%s'", operands[1])
		}
		identity := currentIdentity()
		if len(operands) == 1 && operands[0] != identity.user && operands[0] != accountName() {
			return ExitStatusMessage(1, fmt.Errorf("unknown user %s", operands[0]))
		}
		_, err = fmt.Fprintln(stdout, identity.group)
		return err
	}}
}

// uuidgen makes a name nothing else has, which is what a scratch file or a correlation id
// in a log needs.
//
// Version 4: random apart from the six bits that say which version and variant it is. The
// randomness comes from **crypto/rand**, and that is a correctness question rather than a
// stylistic one -- a seeded generator would make a "unique" identifier that repeats from
// one run to the next, which is the one thing this command must not do.
func newUUIDGenApplet() Applet {
	return simpleApplet{name: "uuidgen", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		if err := refuseArguments(args); err != nil {
			return err
		}
		text, err := randomUUID()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, text)
		return err
	}}
}

func randomUUID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("cannot read random bytes: %v", err)
	}
	// The version nibble and the variant bits, which are what make this a v4 UUID rather
	// than sixteen random bytes wearing the shape of one.
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16]), nil
}

// refuseArguments is the check the commands that take none all need.
func refuseArguments(args []string) error {
	_, operands, err := parseAppletOptions(args, "", "")
	if err != nil {
		return err
	}
	if len(operands) > 0 {
		return fmt.Errorf("extra operand '%s'", operands[0])
	}
	return nil
}
