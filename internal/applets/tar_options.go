package applets

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// tar's options, busybox's: the letters `ctxvzjaOfCkmohTX`, and its long forms, each of a
// letter but for four of its own -- --exclude PATTERN, --strip-components N, --no-recursion and
// --overwrite. --numeric-owner and --no-same-permissions are taken: nothing extracted here has
// an owner or a mode restored to forget.

// tarLongOptions are the long forms that stand for a letter.
var tarLongOptions = map[string]string{
	"list": "t", "extract": "x", "create": "c", "directory": "C", "file": "f", "to-stdout": "O",
	"no-same-owner": "o", "verbose": "v", "keep-old": "k", "dereference": "h", "bzip2": "j",
	"files-from": "T", "exclude-from": "X", "gzip": "z", "auto-compress": "a", "touch": "m",
}

// tarLongValued are the long forms of a letter that takes a value.
var tarLongValued = map[string]bool{"directory": true, "file": true, "files-from": true, "exclude-from": true}

// tarArguments turns the long options into letters, and takes up the four of tar's own.
func tarArguments(args []string, request *tarRequest) ([]string, error) {
	var words []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" || !strings.HasPrefix(arg, "--") {
			if words = append(words, arg); arg == "--" {
				return append(words, args[index+1:]...), nil
			}
			continue
		}
		name, value, valued := strings.Cut(arg[2:], "=")
		takesValue := tarLongValued[name] || name == "exclude" || name == "strip-components"
		if takesValue && !valued {
			if index+1 >= len(args) {
				return nil, fmt.Errorf("option '%s' requires an argument", arg)
			}
			index++
			value, valued = args[index], true
		}
		switch {
		case tarLongOptions[name] != "" && valued == takesValue:
			words = append(words, "-"+tarLongOptions[name])
			if takesValue {
				words = append(words, value)
			}
		case name == "exclude":
			request.selection.reject = append(request.selection.reject, value)
		case name == "strip-components":
			strip, err := strconv.Atoi(value)
			if err != nil || strip < 0 {
				return nil, fmt.Errorf("invalid number '%s'", value)
			}
			request.selection.strip = strip
		case !valued && (name == "no-recursion" || name == "overwrite"):
			request.noRecursion = request.noRecursion || name == "no-recursion"
			request.overwrite = request.overwrite || name == "overwrite"
		case !valued && (name == "numeric-owner" || name == "no-same-permissions"):
		default:
			return nil, fmt.Errorf("unrecognized option '%s'", arg)
		}
	}
	return words, nil
}

// newTarRequest reads tar's arguments into what it was asked: the options, and the names --
// those -T FILE lists first, then the operands, as busybox gathers them -- with the patterns
// -X FILE lists beside --exclude's.
func newTarRequest(ctx context.Context, args []string, stdin io.Reader) (tarRequest, appletOptions, error) {
	request := tarRequest{selection: &tarSelection{}}
	words, err := tarArguments(tarOldStyle(args), &request)
	if err != nil {
		return request, appletOptions{}, err
	}
	options, operands, err := parseAppletOptions(ctx, words, "ctxvzjaOkmoh", "fCTX")
	if err != nil {
		return request, options, err
	}
	request.verbose, request.toStdout = options.has('v'), options.has('O')
	request.gzip, request.bzip2, request.autoDetect = options.has('z'), options.has('j'), options.has('a')
	request.file, request.directory = options.value('f'), options.value('C')
	request.keepOld, request.keepTime, request.dereference = options.has('k'), !options.has('m'), options.has('h')
	for _, list := range options.all('T') {
		names, err := readNameList(ctx, list, stdin)
		if err != nil {
			return request, options, err
		}
		request.operands = append(request.operands, names...)
	}
	for _, operand := range operands {
		request.operands = append(request.operands, tarName(operand))
	}
	for _, list := range options.all('X') {
		patterns, err := readNameList(ctx, list, stdin)
		if err != nil {
			return request, options, err
		}
		request.selection.reject = append(request.selection.reject, patterns...)
	}
	return request, options, nil
}

// readNameList is a -T or -X FILE's lines, each a name or a pattern; `-` is standard input. An
// empty line is a name too, one that is not there, as it is to busybox.
func readNameList(ctx context.Context, name string, stdin io.Reader) ([]string, error) {
	file, err := OpenProcessOperand(ctx, ProcessViewFromContext(ctx), name, stdin)
	if err != nil {
		return nil, cannotOpen(name, err)
	}
	defer file.Close()
	text, err := io.ReadAll(file)
	if err != nil {
		return nil, operandFailure(name, err)
	}
	if len(text) == 0 {
		return nil, nil
	}
	lines := strings.Split(strings.TrimSuffix(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n"), "\n")
	for index, line := range lines {
		lines[index] = tarName(line)
	}
	return lines, nil
}

// tarName is a name as busybox takes it from the command line or a list: a trailing slash taken
// off, unless the slash is all of it.
func tarName(name string) string {
	if len(name) > 1 && name[len(name)-1] == '/' {
		return name[:len(name)-1]
	}
	return name
}
