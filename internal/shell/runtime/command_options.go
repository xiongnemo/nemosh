package runtime

import (
	"fmt"
	"os"
	"path/filepath"
)

// command's options, which are busybox's: -v answers what a name is briefly, -V in type's
// words, and -p looks the command up on a default PATH rather than the shell's. -V and -p
// were taken for command names, so `command -V cd` was "-V: not found". As in busybox, -v
// and -V answer only the first name, and the last of them given wins; bash mode answers every
// name, as bash does. A name that is not
// found is 127, as busybox answers it for both and for type, -V saying `x: not found` on
// stdout as type does; it was bash's 1, and bash's words on stderr.

// commandRequest is what command's options asked for.
type commandRequest struct {
	defaultPath bool
	describe    rune
}

func parseCommandArgs(args []string) (commandRequest, []string, error) {
	var request commandRequest
	for index, arg := range args {
		if arg == "--" {
			return request, args[index+1:], nil
		}
		if len(arg) < 2 || arg[0] != '-' {
			return request, args[index:], nil
		}
		for _, letter := range arg[1:] {
			switch letter {
			case 'p':
				request.defaultPath = true
			case 'v', 'V':
				request.describe = letter
			default:
				return request, nil, fmt.Errorf("illegal option -%c", letter)
			}
		}
	}
	return request, nil, nil
}

// commandVerbose is `command -V name`.
func (r Runtime) commandVerbose(name string) int {
	kinds := r.commandKinds(name)
	if len(kinds) == 0 && r.BashMode() {
		// bash's, which bash mode takes: on stderr, status 1.
		fmt.Fprintf(r.streams.Stderr, "%scommand: %s: not found\n", r.diagnosticPrefix(), name)
		return 1
	}
	if len(kinds) == 0 {
		fmt.Fprintf(r.streams.Stdout, "%s: not found\n", name)
		return 127
	}
	fmt.Fprintln(r.streams.Stdout, kinds[0].description)
	return 0
}

// defaultSearchPath is where `command -p` looks: System32 and the Windows directory, where
// busybox-w32 looks.
func defaultSearchPath() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32") + string(os.PathListSeparator) + root
}
