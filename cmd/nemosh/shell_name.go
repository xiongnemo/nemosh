package main

// sh and bash are this shell's names, as busybox-w32 is its ash under either: a nemosh named
// bash.exe, and `nemosh bash ARGS`, are bash reading ARGS. That second spelling is what the shell
// runs for a command named bash, and for `#!/bin/bash` (the runtime's shell_fallback.go), as
// busybox runs `busybox bash` for its own. Run as bash, BASH_VERSION and BASH_VERSINFO are set;
// under either name, $0 is that name where it would be nemosh. In `nemosh bash ARGS` an operand
// that is an applet's name is a script, as it is to sh, where `nemosh NAME` runs the applet. A
// nemosh named bash.exe still reads `bash.exe NAME` as the applet or the shell NAME first, since
// that is how it launches itself; Oils' harness installs it so, and its `$SH -c` and its `#!/bin/cat`
// were `cannot open 'bash'` and `cannot open 'cat'` while it read its own name first.

// shellName is whether name is one of this shell's.
func shellName(name string) bool {
	return name == "sh" || name == "bash"
}

// scriptName is $0 when nothing else names the shell: the name it was run by, or nemosh.
func (c command) scriptName() string {
	if c.name != "" {
		return c.name
	}
	return defaultScriptName
}
