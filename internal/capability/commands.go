package capability

// The applet rows are transcribed from docs/support-matrix.md, which is itself
// measured against a built binary rather than read off the source, and are then
// bound to behaviour by capability_test.go. A row that overstates what an applet
// takes fails there; so does one that understates it.
//
// ValueShort had been understated on five rows -- cut, date, head, mkdir and su --
// which is not cosmetic: it is what completion reads to know that the word after
// `-d` is a delimiter rather than a file, so Tab was offering directory listings
// after `head -n` and after `su -c`. Measured, all five take the value as the next
// word. Found by the usage test next door rather than by capability_test.go, which
// binds Short to behaviour and does not yet bind ValueShort.
//
// Directory is claimed only where a regular file could never have been meant:
// `cd notes.txt`, `mkdir notes.txt` and `rmdir notes.txt` all fail outright.
// Everything else is AnyPath, including commands that take no operand at all --
// offering a path there is harmless, and refusing one would hide a file the user
// meant to name.
var commands = []Command{
	{Name: "base64", Short: "diw", ValueShort: "w", Operand: AnyPath},
	{Name: "basename", Short: "a", Operand: AnyPath},
	{Name: "cat", Short: "n", Operand: AnyPath},
	{Name: "chmod", Operand: AnyPath},
	{Name: "clear", Operand: AnyPath},
	{Name: "cmp", Short: "sl", Operand: AnyPath},
	{Name: "comm", Short: "123", Operand: AnyPath},
	{Name: "cp", Short: "rR", Operand: AnyPath},
	{Name: "cut", Short: "bcdfns", ValueShort: "bcdf", Operand: AnyPath},
	{Name: "date", Short: "du", ValueShort: "d", Operand: AnyPath},
	{Name: "dirname", Operand: AnyPath},
	{Name: "echo", Short: "ne", Operand: AnyPath},
	{Name: "env", Short: "i", Operand: AnyPath},
	{Name: "du", Short: "sh", Operand: AnyPath},
	{Name: "expr", Operand: AnyPath},
	{Name: "find", Operand: AnyPath},
	{Name: "grep", Short: "invrRlcqwxFoshHEm", ValueShort: "m", Long: []string{"color"}, Operand: AnyPath},
	{Name: "head", Short: "nc", ValueShort: "nc", Operand: AnyPath},
	{Name: "id", Short: "ugGn", Operand: AnyPath},
	{Name: "ln", Short: "s", Operand: AnyPath},
	{Name: "ls", Short: "ahl1", Long: []string{"color"}, Operand: AnyPath},
	{Name: "mkdir", Short: "mpv", ValueShort: "m", Operand: Directory},
	{Name: "mktemp", Short: "dqu", Operand: AnyPath},
	{Name: "mv", Short: "f", Operand: AnyPath},
	{Name: "nl", Short: "b", ValueShort: "b", Operand: AnyPath},
	{Name: "paste", Short: "sd", ValueShort: "d", Operand: AnyPath},
	{Name: "pgrep", Short: "lx", Operand: AnyPath},
	{Name: "pkill", Short: "x", Operand: AnyPath},
	{Name: "posixpath", Operand: AnyPath},
	{Name: "printenv", Operand: AnyPath},
	{Name: "printf", Operand: AnyPath},
	{Name: "ps", Operand: AnyPath},
	{Name: "top", Short: "bndsfoHt", ValueShort: "ndsfo", Operand: AnyPath},
	{Name: "pwd", Short: "LP", Operand: AnyPath},
	{Name: "readlink", Short: "n", Operand: AnyPath},
	{Name: "rev", Operand: AnyPath},
	{Name: "realpath", Operand: AnyPath},
	{Name: "rm", Short: "fr", Operand: AnyPath},
	{Name: "rmdir", Short: "pv", Operand: Directory},
	{Name: "md5sum", Short: "bctw", Operand: AnyPath},
	{Name: "sed", Operand: AnyPath},
	{Name: "sha256sum", Short: "bctw", Operand: AnyPath},
	{Name: "seq", Operand: AnyPath},
	{Name: "sleep", Operand: AnyPath},
	{Name: "sort", Short: "nrufbkt", ValueShort: "kt", Operand: AnyPath},
	{Name: "stat", Short: "c", ValueShort: "c", Operand: AnyPath},
	{Name: "split", Short: "l", ValueShort: "l", Operand: AnyPath},
	// su's operand is a user name rather than a path, and the only name it takes
	// is `root`. AnyPath anyway: completion offering a file there is harmless,
	// and there is no kind for "one fixed word".
	{Name: "su", Short: "cstWN", ValueShort: "cs", Operand: AnyPath},
	{Name: "tac", Operand: AnyPath},
	{Name: "tail", Short: "nc", ValueShort: "nc", Operand: AnyPath},
	{Name: "test", Operand: AnyPath},
	{Name: "[", Operand: AnyPath},
	{Name: "tee", Short: "a", Operand: AnyPath},
	{Name: "touch", Short: "c", Operand: AnyPath},
	{Name: "tr", Short: "dsc", Operand: AnyPath},
	{Name: "true", Operand: AnyPath},
	{Name: "false", Operand: AnyPath},
	{Name: "uname", Short: "aimnoprsv", Operand: AnyPath},
	{Name: "uniq", Short: "cdui", Operand: AnyPath},
	{Name: "wc", Short: "clwmL", Operand: AnyPath},
	{Name: "whoami", Operand: AnyPath},
	{Name: "winpath", Operand: AnyPath},
	{Name: "xargs", Short: "0rtnI", ValueShort: "nI", Operand: AnyPath},
	{Name: "xxd", Short: "p", Operand: AnyPath},
	{Name: "yes", Operand: AnyPath},

	// Builtins carry an operand kind and no option claims. Nothing here measures
	// a builtin's options, and a claim that is not checked is the kind that goes
	// stale silently -- the whole reason this table exists in one place.
	//
	// The names are checked against the shell's own list by a test, so a builtin
	// added or removed there cannot leave this behind.
	{Name: ":", Operand: AnyPath, Builtin: true},
	{Name: ".", Operand: AnyPath, Builtin: true},
	{Name: "alias", Operand: AnyPath, Builtin: true},
	{Name: "break", Operand: AnyPath, Builtin: true},
	{Name: "cd", Operand: Directory, Builtin: true},
	{Name: "command", Operand: AnyPath, Builtin: true},
	{Name: "declare", Operand: AnyPath, Builtin: true},
	{Name: "continue", Operand: AnyPath, Builtin: true},
	{Name: "eval", Operand: AnyPath, Builtin: true},
	{Name: "exec", Operand: AnyPath, Builtin: true},
	{Name: "exit", Operand: AnyPath, Builtin: true},
	{Name: "export", Operand: AnyPath, Builtin: true},
	{Name: "getopts", Operand: AnyPath, Builtin: true},
	{Name: "help", Operand: AnyPath, Builtin: true},
	{Name: "history", Operand: AnyPath, Builtin: true},
	{Name: "jobs", Operand: AnyPath, Builtin: true},
	{Name: "kill", Operand: AnyPath, Builtin: true},
	{Name: "let", Operand: AnyPath, Builtin: true},
	{Name: "local", Operand: AnyPath, Builtin: true},
	{Name: "read", Operand: AnyPath, Builtin: true},
	{Name: "readonly", Operand: AnyPath, Builtin: true},
	{Name: "return", Operand: AnyPath, Builtin: true},
	{Name: "set", Operand: AnyPath, Builtin: true},
	{Name: "shift", Operand: AnyPath, Builtin: true},
	{Name: "shopt", Operand: AnyPath, Builtin: true},
	{Name: "source", Operand: AnyPath, Builtin: true},
	{Name: "times", Operand: AnyPath, Builtin: true},
	{Name: "trap", Operand: AnyPath, Builtin: true},
	{Name: "type", Operand: AnyPath, Builtin: true},
	{Name: "typeset", Operand: AnyPath, Builtin: true},
	{Name: "umask", Operand: AnyPath, Builtin: true},
	{Name: "unalias", Operand: AnyPath, Builtin: true},
	{Name: "unset", Operand: AnyPath, Builtin: true},
	{Name: "wait", Operand: AnyPath, Builtin: true},
	{Name: "which", Operand: AnyPath, Builtin: true},
}
