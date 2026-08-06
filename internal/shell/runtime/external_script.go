package runtime

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
)

// interpreterHeaderSize is busybox's window: it reads sizeof(interp->buf)-1 with
// a 100-byte buffer (include/mingw.h:598), so a shebang line that does not end
// inside 99 bytes is not a shebang at all.
const interpreterHeaderSize = 99

const (
	defaultInterpreterPath = "/bin/sh"
	defaultInterpreterName = "sh"
	shellExecutableName    = "nemosh"
)

// maxInterpreterDepth matches busybox's `++level > 4` guard (win32/process.c:314)
// and stops a script that names itself, directly or in a ring.
const maxInterpreterDepth = 4

var errInterpreterLoop = errors.New("too many levels of interpreter")

type interpreter struct {
	path string
	name string
	opts string
}

// parseInterpreter reproduces busybox-w32 parse_interpreter (win32/process.c:66).
func parseInterpreter(path string) (interpreter, bool, error) {
	header, err := readInterpreterHeader(path)
	if err != nil {
		return interpreter{}, false, err
	}
	if found, ok := parseShebang(header); ok {
		return found, true, nil
	}
	// A .sh file with no usable shebang is still a shell script. busybox applies
	// this fallback even when the first line looked like a shebang but was too
	// long or unterminated, and so does this.
	if hasShellScriptSuffix(path) {
		return interpreter{path: defaultInterpreterPath, name: defaultInterpreterName}, true, nil
	}
	return interpreter{}, false, nil
}

func readInterpreterHeader(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read script %q: %w", path, err)
	}
	defer file.Close()
	header := make([]byte, interpreterHeaderSize)
	count, err := io.ReadFull(file, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("read script %q: %w", path, err)
	}
	return header[:count], nil
}

func parseShebang(header []byte) (interpreter, bool) {
	// busybox refuses to judge fewer than four bytes: '#!/x' is the shortest
	// shebang that can name anything.
	if len(header) < 4 || header[0] != '#' || header[1] != '!' {
		return interpreter{}, false
	}
	newline := bytes.IndexByte(header, '\n')
	if newline < 0 {
		return interpreter{}, false
	}
	path, rest := cutInterpreterToken(string(header[2:newline]))
	if path == "" {
		return interpreter{}, false
	}
	name := path
	if index := strings.LastIndexAny(path, `/\`); index >= 0 {
		name = path[index+1:]
	}
	if name == "" {
		return interpreter{}, false
	}
	return interpreter{path: path, name: name, opts: interpreterOption(rest)}, true
}

// cutInterpreterToken takes the first token the way strtok(buf+2, " \t\r\n")
// does: leading separators are skipped, and the token ends at the next one.
func cutInterpreterToken(line string) (string, string) {
	const separators = " \t\r"
	trimmed := strings.TrimLeft(line, separators)
	end := strings.IndexAny(trimmed, separators)
	if end < 0 {
		return trimmed, ""
	}
	return trimmed[:end], trimmed[end:]
}

// interpreterOption takes everything left as a single argument, which is what
// Linux does too: `#!/bin/sh -x -y` passes "-x -y" as one word, not two. An
// option that trims away to nothing is no option at all.
func interpreterOption(rest string) string {
	if index := strings.IndexByte(rest, '\r'); index >= 0 {
		rest = rest[:index]
	}
	return strings.TrimSpace(rest)
}

func hasShellScriptSuffix(path string) bool {
	return len(path) >= 3 && strings.EqualFold(path[len(path)-3:], ".sh")
}

// unixInterpreterPath mirrors busybox unix_path (win32/mingw.c:2569). Only these
// four directories are treated as naming the Unix world rather than the
// filesystem, which is what makes #!/bin/sh mean "the shell" on a machine that
// has no /bin.
func unixInterpreterPath(path string) bool {
	index := strings.LastIndex(path, "/")
	if index < 0 {
		return false
	}
	switch path[:index] {
	case "/bin", "/usr/bin", "/sbin", "/usr/sbin":
		return true
	default:
		return false
	}
}

// externalLaunchTarget rewrites a launch when the resolved file is a script.
// Windows cannot run one directly, so the interpreter becomes the program and
// the script becomes its first operand. Everywhere else the kernel reads the
// shebang itself and this stays out of the way.
func (r Runtime) externalLaunchTarget(executable string, args []string) (string, []string, error) {
	if runtime.GOOS != "windows" {
		return executable, args, nil
	}
	self, err := os.Executable()
	if err != nil {
		return "", nil, fmt.Errorf("locate the nemosh executable: %w", err)
	}
	return r.planScriptLaunch(executable, args, self)
}

// planScriptLaunch takes the nemosh executable as a parameter rather than asking
// for it, so the rewrite can be tested without os.Executable naming whichever
// binary happens to be running.
func (r Runtime) planScriptLaunch(executable string, args []string, self string) (string, []string, error) {
	for depth := 0; ; depth++ {
		interp, found, err := parseInterpreter(executable)
		if err != nil {
			return "", nil, err
		}
		if !found {
			return executable, args, nil
		}
		if depth >= maxInterpreterDepth {
			return "", nil, fmt.Errorf("%s: %w", executable, errInterpreterLoop)
		}
		next, applet, err := r.interpreterExecutable(interp)
		if err != nil {
			return "", nil, err
		}
		if next == "" {
			next = self
		}
		// busybox rebuilds argv as [interpreter, opts?, absolute script, args...]
		// and drops the caller's argv[0] (win32/process.c:320).
		rebuilt := make([]string, 0, len(args)+3)
		if applet != "" {
			rebuilt = append(rebuilt, applet)
		}
		if interp.opts != "" {
			rebuilt = append(rebuilt, interp.opts)
		}
		rebuilt = append(rebuilt, executable)
		executable, args = next, append(rebuilt, args...)
		if executable == self {
			return executable, args, nil
		}
	}
}

// interpreterExecutable answers with the program to run and, when the
// interpreter names an applet, the applet to hand it. An empty program means
// nemosh itself. The order is busybox's (win32/process.c:325): an applet under a
// Unix directory wins, then the path as written, then a PATH search by name.
func (r Runtime) interpreterExecutable(interp interpreter) (string, string, error) {
	unix := unixInterpreterPath(interp.path)
	if unix {
		// nemosh has no sh applet -- it is the shell -- so /bin/sh is answered by
		// running this binary against the script, which is also the new process
		// POSIX requires for a script executed as a command.
		if interp.name == defaultInterpreterName || interp.name == shellExecutableName {
			return "", "", nil
		}
		if _, ok := r.lookupApplet(interp.name); ok {
			return "", interp.name, nil
		}
	}
	resolved, err := r.externalCommandPath(interp.path)
	if err == nil {
		return resolved, "", nil
	}
	if unix {
		if byName, nameErr := r.externalCommandPath(interp.name); nameErr == nil {
			return byName, "", nil
		}
	}
	return "", "", fmt.Errorf("%s: %w", interp.path, err)
}
