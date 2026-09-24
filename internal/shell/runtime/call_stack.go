package runtime

import (
	"fmt"
	"strconv"
)

// The call stack bash exposes: $FUNCNAME as an array, $BASH_SOURCE, $BASH_LINENO and the
// `caller` builtin. busybox has only the scalar $FUNCNAME, so bash is the reference.
//
// `SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)` is among the commonest lines
// in a bash script, and $BASH_SOURCE was unset: dirname of nothing is `.`, so the script
// quietly took the directory it was run from for the one it lives in.

// callFrame is one level of what is running: a function call, or a file being sourced.
// Never changed once made, and linked outward, so a Runtime value carries its own stack
// and a call's frame is gone when the caller's copy is back in charge.
type callFrame struct {
	// name is the function, or "source" for a sourced file.
	name string
	// file is the file this frame's code is in: where the function was defined, or the
	// file being sourced.
	file string
	// line is the line in the frame outside this one where this one was entered.
	line  int
	outer *callFrame
}

// SetScriptFile names the file the shell runs, which is $BASH_SOURCE at the top. A command
// string and standard input have none.
func (r *Runtime) SetScriptFile(path string) { r.scriptFile = path }

// currentFile is the file the running code is in.
func (r Runtime) currentFile() string {
	if r.frames != nil {
		return r.frames.file
	}
	return r.scriptFile
}

// enterFrame is r with one more level on its stack, entered from the running line.
func (r Runtime) enterFrame(name, file string) Runtime {
	r.frames = &callFrame{name: name, file: file, line: r.currentLine(), outer: r.frames}
	return r
}

// callStack is the three arrays, innermost first. The script's own level is last, when
// there is a script, as `main`. FUNCNAME is empty outside a function, as bash has it:
// a sourced file at the top level is a level of BASH_SOURCE and not of FUNCNAME.
func (r Runtime) callStack() (names, files, lines []string) {
	inFunction := false
	for frame := r.frames; frame != nil; frame = frame.outer {
		names = append(names, frame.name)
		files = append(files, frame.file)
		lines = append(lines, strconv.Itoa(frame.line))
		inFunction = inFunction || frame.name != "source"
	}
	if r.scriptFile != "" {
		names, files, lines = append(names, "main"), append(files, r.scriptFile), append(lines, "0")
	}
	if !inFunction {
		names = nil
	}
	return names, files, lines
}

// callStackArray answers FUNCNAME, BASH_SOURCE and BASH_LINENO as arrays.
func (r Runtime) callStackArray(name string) ([]string, bool) {
	names, files, lines := r.callStack()
	switch name {
	case "FUNCNAME":
		return names, true
	case "BASH_SOURCE":
		return files, true
	case "BASH_LINENO":
		return lines, true
	}
	return nil, false
}

// caller is bash's: `caller` says the line and file a function was called from, and
// `caller N` the line, function and file N levels out -- what a die() uses to say where it
// was called. Past the outermost level it says nothing and fails.
func (r Runtime) caller(args []string) int {
	_, files, lines := r.callStack()
	names := r.frameNames()
	if len(args) == 0 {
		if len(lines) < 2 {
			fmt.Fprintln(r.streams.Stdout, "0 NULL")
			return 0
		}
		fmt.Fprintf(r.streams.Stdout, "%s %s\n", lines[0], files[1])
		return 0
	}
	level, err := strconv.Atoi(args[0])
	if err != nil || level < 0 {
		fmt.Fprintf(r.streams.Stderr, "caller: %s: invalid number\n", args[0])
		return 2
	}
	if level+1 >= len(lines) {
		return 1
	}
	fmt.Fprintf(r.streams.Stdout, "%s %s %s\n", lines[level], names[level+1], files[level+1])
	return 0
}

// frameNames is FUNCNAME whether or not a function is running, which caller needs for a
// sourced file's levels.
func (r Runtime) frameNames() []string {
	var names []string
	for frame := r.frames; frame != nil; frame = frame.outer {
		names = append(names, frame.name)
	}
	if r.scriptFile != "" {
		names = append(names, "main")
	}
	return names
}
