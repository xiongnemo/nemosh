package runtime

import (
	"fmt"
	"os"
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// bind is bash's, which busybox has not got: `bind [-lpsvPSVX] [-m keymap] [-f file] [-q name]
// [-u name] [-r keyseq] [-x keyseq:command] [line ...]`. Its options are read first and done in
// bash's order -- the keymap, the listings, -f, -q, -u, -r, -x -- and then each other argument
// as a line of readline's init syntax (bind_line.go). It works in a script as in a session, as
// bash's does once it has warned; the line editor reads what it set (readline_bindings.go).
// It was refused as not implemented, which left fzf's key bindings and atuin's without a key.

const bindUsage = "bind: usage: bind [-lpsvPSVX] [-m keymap] [-f filename] [-q name] [-u name] [-r keyseq] [-x keyseq:shell-command] [keyseq:readline-function or readline-command]"

type bindRequest struct {
	keymap, prefix string
	list           string
	file, query    string
	unbind, remove []string
	commands       []string
	lines          []string
}

func (r Runtime) bindBuiltin(args []string) int {
	request, status, ok := r.readBindOptions(args)
	if !ok {
		return status
	}
	for _, flag := range request.list {
		switch flag {
		case 'l':
			for _, name := range strings.Fields(readlineFunctionNames) {
				fmt.Fprintln(r.streams.Stdout, name)
			}
		case 'p', 'P':
			r.bindListFunctions(request.keymap, flag == 'p')
		case 's', 'S':
			r.bindListTexts(request.keymap, BindMacro, flag == 's')
		case 'X':
			r.bindListTexts(request.keymap, BindCommand, true)
		case 'v', 'V':
			r.bindListVariables(request.keymap, flag == 'v')
		}
	}
	if request.file != "" {
		status = max(status, r.bindFile(request.keymap, request.prefix, request.file))
	}
	if request.query != "" {
		status = max(status, r.bindQuery(request.keymap, request.query))
	}
	for _, name := range request.unbind {
		for _, keys := range keysBound(r.bindings.bound(request.keymap), functionBound(name)) {
			parsed, _ := parseKeyseq(keys)
			r.bindings.set(request.keymap, parsed, KeyBinding{Kind: BindRemoved})
		}
	}
	for _, keys := range request.remove {
		if parsed, err := parseKeyseq(strings.Trim(keys, `"`)); err == nil {
			r.bindings.set(request.keymap, append([]byte(request.prefix), parsed...), KeyBinding{Kind: BindRemoved})
		}
	}
	for _, command := range request.commands {
		if err := r.bindCommand(request.keymap, request.prefix, command); err != nil {
			status = r.bindFailed(err)
		}
	}
	for _, line := range request.lines {
		if err := r.bindLine(request.keymap, request.prefix, line); err != nil {
			status = r.bindFailed(err)
		}
	}
	return status
}

func (r Runtime) bindFailed(err error) int {
	fmt.Fprintf(r.streams.Stderr, "%sbind: %v\n", r.diagnosticPrefix(), err)
	return 1
}

// readBindOptions reads bind's options, getopt's way, and the lines after them.
func (r Runtime) readBindOptions(args []string) (bindRequest, int, bool) {
	request := bindRequest{keymap: r.defaultKeymap()}
	index := 0
	for ; index < len(args) && len(args[index]) > 1 && args[index][0] == '-'; index++ {
		if args[index] == "--" {
			index++
			break
		}
		word := args[index]
		for at := 1; at < len(word); at++ {
			letter := word[at]
			if strings.IndexByte("lpsvPSVX", letter) >= 0 {
				request.list += string(letter)
				continue
			}
			if strings.IndexByte("mfqurx", letter) < 0 {
				fmt.Fprintf(r.streams.Stderr, "%sbind: -%c: invalid option\n%s\n", r.diagnosticPrefix(), letter, bindUsage)
				return request, 2, false
			}
			value := word[at+1:]
			if value == "" {
				if index+1 == len(args) {
					fmt.Fprintf(r.streams.Stderr, "%sbind: -%c: option requires an argument\n%s\n", r.diagnosticPrefix(), letter, bindUsage)
					return request, 2, false
				}
				index++
				value = args[index]
			}
			if !r.takeBindOption(&request, letter, value) {
				return request, 1, false
			}
			break
		}
	}
	request.lines = args[index:]
	return request, 0, true
}

// takeBindOption records one option that takes a value.
func (r Runtime) takeBindOption(request *bindRequest, letter byte, value string) bool {
	switch letter {
	case 'm':
		keymap, prefix, ok := keymapNamed(value)
		if !ok {
			fmt.Fprintf(r.streams.Stderr, "%sbind: `%s': invalid keymap name\n", r.diagnosticPrefix(), value)
			return false
		}
		request.keymap, request.prefix = keymap, prefix
	case 'f':
		request.file = value
	case 'q':
		request.query = value
	case 'u':
		request.unbind = append(request.unbind, value)
	case 'r':
		request.remove = append(request.remove, value)
	case 'x':
		request.commands = append(request.commands, value)
	}
	return true
}

// defaultKeymap is the keymap bind changes when -m names none: the one the editor is in.
func (r Runtime) defaultKeymap() string {
	if r.options.vi {
		return "vi-insert"
	}
	return "emacs"
}

// bindFile is -f: a file read as an inputrc is (inputrc.go).
func (r Runtime) bindFile(keymap, prefix, path string) int {
	data, err := os.ReadFile(r.resolvePath(path))
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%sbind: %s: cannot read: %s\n", r.diagnosticPrefix(), path, applets.CauseText(err))
		return 1
	}
	return r.bindInputrc(keymap, prefix, string(data), 0)
}
