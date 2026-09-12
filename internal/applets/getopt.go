package applets

import (
	"fmt"
	"io"
	"strings"
)

// getopt canonicalises a command line so a shell script can read it with a simple loop.
//
// It exists because parsing options in shell is otherwise miserable: `-abc` has to become
// `-a -b -c`, `--file=x` has to become `--file x`, and the operands have to end up after a
// `--` however they were written. This does that once and prints the result for `eval set
// -- "$(getopt ...)"` to consume.
//
// **The output is quoted**, which is the whole reason `eval` is safe here: an operand with
// a space in it comes back as one word. `-u` turns the quoting off, and a script using it
// is asking for the old behaviour where a file called `My Documents` becomes two arguments.
//
// An unrecognised option is a **diagnostic and a non-zero status, but the canonical line is
// still printed** -- with what was understood. That is what the references do, and a script
// that checks the status before calling `eval` sees the failure either way.
func newGetoptApplet() Applet {
	return simpleApplet{name: "getopt", run: func(args []string, _ io.Reader, stdout, stderr io.Writer) error {
		request, err := parseGetoptRequest(args)
		if err != nil {
			return err
		}
		if request.versionTest {
			// -T is how a script asks whether this is the enhanced getopt. Answering 4
			// says yes, and prints nothing.
			return ExitStatus(4)
		}
		canonical, failure := request.canonicalise(stderr)
		if !request.silentOutput {
			if _, err := fmt.Fprintln(stdout, canonical); err != nil {
				return err
			}
		}
		if failure {
			return ExitStatus(1)
		}
		return nil
	}}
}

type getoptRequest struct {
	shortOptions string
	longOptions  []string
	name         string
	quiet        bool
	silentOutput bool
	unquoted     bool
	allowSingle  bool
	versionTest  bool
	parameters   []string
}

func parseGetoptRequest(args []string) (getoptRequest, error) {
	request := getoptRequest{name: "getopt"}
	sawShort := false
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
		value, consumed, err := getoptOptionValue(args, index, argument)
		if err != nil {
			return request, err
		}
		index += consumed
		switch {
		case argument == "-o" || strings.HasPrefix(argument, "-o") || argument == "--options":
			request.shortOptions, sawShort = value, true
		case argument == "-l" || argument == "--long" || argument == "--longoptions":
			request.longOptions = append(request.longOptions, strings.Split(value, ",")...)
		case argument == "-n" || argument == "--name":
			request.name = value
		case argument == "-s" || argument == "--shell":
			if value != "sh" && value != "bash" {
				// csh quoting is a different language and this shell is not it.
				return request, fmt.Errorf("only the sh and bash quoting conventions are supported, not %s", value)
			}
		case argument == "-q" || argument == "--quiet":
			request.quiet = true
		case argument == "-Q" || argument == "--quiet-output":
			request.silentOutput = true
		case argument == "-u" || argument == "--unquoted":
			request.unquoted = true
		case argument == "-a" || argument == "--alternative":
			request.allowSingle = true
		case argument == "-T" || argument == "--test":
			request.versionTest = true
		default:
			return request, fmt.Errorf("unsupported option: %s", argument)
		}
	}
	rest := args[index:]
	if !sawShort {
		// The old form: the first operand is the option string. This is why `getopt ab:
		// -a` works without a -o.
		if len(rest) == 0 {
			if request.versionTest {
				return request, nil
			}
			return request, missingOperand()
		}
		request.shortOptions, rest = rest[0], rest[1:]
		// The old form does not quote. That is not an oversight in the references: a
		// script written for the original getopt splits the output on blanks itself, and
		// quoting would hand it a word with quotes still in it.
		request.unquoted = true
	}
	// A leading `+` or `-` in the option string is POSIXLY_CORRECT / in-order scanning,
	// neither of which changes what this prints.
	request.shortOptions = strings.TrimLeft(request.shortOptions, "+-")
	request.parameters = rest
	return request, nil
}

// getoptOptionValue answers the value for an option that takes one, joined or separate.
func getoptOptionValue(args []string, index int, argument string) (string, int, error) {
	takesValue := map[string]bool{
		"-o": true, "-l": true, "-n": true, "-s": true,
		"--options": true, "--long": true, "--longoptions": true, "--name": true, "--shell": true,
	}
	if name, value, joined := strings.Cut(argument, "="); joined && takesValue[name] {
		return value, 0, nil
	}
	if len(argument) > 2 && argument[1] == 'o' && argument[0] == '-' {
		return argument[2:], 0, nil
	}
	if !takesValue[argument] {
		return "", 0, nil
	}
	if index+1 >= len(args) {
		return "", 0, fmt.Errorf("option requires an argument -- %s", strings.TrimLeft(argument, "-"))
	}
	return args[index+1], 1, nil
}

// canonicalise is the work: rewrite the parameters into one option per word, then `--`,
// then the operands.
func (r getoptRequest) canonicalise(stderr io.Writer) (string, bool) {
	var options, operands []string
	failed := false
	index := 0
	for ; index < len(r.parameters); index++ {
		word := r.parameters[index]
		switch {
		case word == "--":
			index++
			operands = append(operands, r.parameters[index:]...)
			index = len(r.parameters)
		case strings.HasPrefix(word, "--") || (r.allowSingle && r.looksLong(word)):
			used, bad := r.canonicaliseLong(word, r.parameters, index, &options, stderr)
			index += used
			failed = failed || bad
		case len(word) > 1 && word[0] == '-':
			used, bad := r.canonicaliseShort(word, r.parameters, index, &options, stderr)
			index += used
			failed = failed || bad
		default:
			// An operand. Everything after it is still scanned for options, which is
			// GNU's permuting behaviour and what the references do.
			operands = append(operands, word)
		}
	}
	words := append(options, "--")
	for _, operand := range operands {
		words = append(words, r.quote(operand))
	}
	return " " + strings.Join(words, " "), failed
}

// looksLong decides whether `-name` is a long option written with one dash, which is what
// -a allows.
func (r getoptRequest) looksLong(word string) bool {
	name := strings.TrimPrefix(word, "-")
	name, _, _ = strings.Cut(name, "=")
	for _, long := range r.longOptions {
		if strings.TrimRight(long, ":") == name {
			return true
		}
	}
	return false
}

func (r getoptRequest) canonicaliseLong(word string, all []string, index int, options *[]string, stderr io.Writer) (int, bool) {
	name, value, joined := strings.Cut(strings.TrimLeft(word, "-"), "=")
	for _, long := range r.longOptions {
		bare := strings.TrimRight(long, ":")
		if bare != name {
			continue
		}
		*options = append(*options, "--"+bare)
		switch {
		case joined:
			*options = append(*options, r.quote(value))
		case strings.HasSuffix(long, "::"):
			// An optional argument is only taken when it was joined with `=`, which is
			// the rule that stops `--colour ls` eating the operand.
			*options = append(*options, r.quote(""))
		case strings.HasSuffix(long, ":"):
			if index+1 >= len(all) {
				r.report(stderr, "option '--%s' requires an argument", bare)
				return 0, true
			}
			*options = append(*options, r.quote(all[index+1]))
			return 1, false
		}
		return 0, false
	}
	r.report(stderr, "unknown option -- %s", name)
	return 0, true
}

func (r getoptRequest) canonicaliseShort(word string, all []string, index int, options *[]string, stderr io.Writer) (int, bool) {
	failed := false
	for position := 1; position < len(word); position++ {
		letter := word[position]
		takes := strings.Index(r.shortOptions, string(letter))
		if takes < 0 {
			r.report(stderr, "unknown option -- %c", letter)
			failed = true
			continue
		}
		*options = append(*options, "-"+string(letter))
		wants := takes+1 < len(r.shortOptions) && r.shortOptions[takes+1] == ':'
		if !wants {
			continue
		}
		// The value is the rest of this word if there is any, otherwise the next one:
		// `-bval` and `-b val` mean the same thing.
		if position+1 < len(word) {
			*options = append(*options, r.quote(word[position+1:]))
			return 0, failed
		}
		optional := takes+2 < len(r.shortOptions) && r.shortOptions[takes+2] == ':'
		if optional {
			*options = append(*options, r.quote(""))
			continue
		}
		if index+1 >= len(all) {
			r.report(stderr, "option requires an argument -- %c", letter)
			return 0, true
		}
		*options = append(*options, r.quote(all[index+1]))
		return 1, failed
	}
	return 0, failed
}

func (r getoptRequest) report(stderr io.Writer, format string, args ...any) {
	if r.quiet {
		return
	}
	fmt.Fprintf(stderr, "%s: %s\n", r.name, fmt.Sprintf(format, args...))
}

// quote wraps a word so the shell reads it back as one, unless -u asked otherwise.
func (r getoptRequest) quote(word string) string {
	if r.unquoted {
		return word
	}
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}
