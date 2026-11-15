package capability

import "sync"

// Usage text for the builtins, which `help NAME` and `NAME --help` print, as bash's do.
//
// A builtin's row carries option letters only where completion needs them, so most of what a
// builtin takes is said in its notes rather than drawn from the row; where a row does carry
// letters, each has a gloss here, so the rendered help has no blank lines. The completion
// builtins, whose options are bash's long list, are left to docs/support-matrix.md.
//
// Built on first use rather than at start: a map literal this size is built before main runs,
// and it took package init past the ceiling product.yml holds it to -- 2825 allocations
// against 2750 -- for text that only `help NAME` reads.
var builtinUsage = sync.OnceValue(func() map[string]Usage { return usageTextBuiltins() })

func usageTextBuiltins() map[string]Usage {
	return map[string]Usage{
		":":         {Summary: "Do nothing, successfully: the null command.", Operands: "[ARG]...", Notes: []string{"Its arguments are expanded, so `: ${x:=default}` assigns."}},
		".":         {Summary: "Run the commands in a file in this shell.", Operands: "FILE [ARG]...", Notes: []string{"A FILE with no slash is looked for on PATH first, then where it stands; `shopt -u sourcepath` leaves PATH out."}},
		"source":    {Summary: "Run the commands in a file in this shell; the same as `.`.", Operands: "FILE [ARG]..."},
		"alias":     {Summary: "Define or list aliases.", Operands: "[-p] [NAME[=VALUE]]...", Notes: []string{"With no operands, or -p, every alias is listed in a form that can be read back."}},
		"unalias":   {Summary: "Remove aliases.", Operands: "[-a] NAME...", Notes: []string{"-a removes them all."}},
		"break":     {Summary: "Leave the innermost loop, or the Nth one out.", Operands: "[N]"},
		"continue":  {Summary: "Go on to the next pass of the innermost loop, or the Nth one out.", Operands: "[N]"},
		"builtin":   {Summary: "Run a builtin, passing over a function of the same name.", Operands: "BUILTIN [ARG]..."},
		"caller":    {Summary: "Say which line and file called the function running now.", Operands: "[N]"},
		"cd":        {Summary: "Change the working directory.", Operands: "[-L|-P] [DIRECTORY|-]", Notes: []string{"With no operand it goes to $HOME, and `cd -` to $OLDPWD, printing it. -P resolves links.", "A relative DIRECTORY is looked for under each entry of CDPATH."}},
		"command":   {Summary: "Run a command, passing over functions, or say what it is.", Operands: "[-pVv] COMMAND [ARG]...", Notes: []string{"-v prints the command or the path it would run, -V a sentence about it, and -p searches a default PATH."}},
		"coproc":    {Summary: "Run a command in the background with pipes to and from it.", Operands: "[NAME] COMMAND", Notes: []string{"The shell's ends are ${NAME[1]} for writing and ${NAME[0]} for reading; $NAME_PID is the job."}},
		"declare":   {Summary: "Set variables and their attributes, or show them.", Operands: "[-aAfFgilnprux] [NAME[=VALUE]]...", Notes: []string{"-a and -A make arrays, -i an integer, -n a name reference, -r readonly, -x exported, -l and -u lower and upper case; -p shows them in a form that can be read back."}},
		"typeset":   {Summary: "Set variables and their attributes; the same as declare.", Operands: "[-aAfFgilnprux] [NAME[=VALUE]]..."},
		"local":     {Summary: "Make variables local to the function running now.", Operands: "[NAME[=VALUE]]..."},
		"dirs":      {Summary: "Show the directory stack.", Operands: "[+N|-N]", Options: map[string]string{"c": "clear the stack", "l": "print the home directory in full rather than as ~", "p": "one entry per line", "v": "one entry per line, numbered"}},
		"pushd":     {Summary: "Change directory and put it on the directory stack.", Operands: "[DIRECTORY|+N|-N]"},
		"popd":      {Summary: "Take a directory off the stack and change to the one below it.", Operands: "[+N|-N]"},
		"disown":    {Summary: "Take jobs out of the job table.", Operands: "[JOB|PID]...", Options: map[string]string{"a": "every job", "r": "the running jobs", "h": "keep the job: this shell sends no hangup when it exits anyway"}, Notes: []string{"With no operand, the current job. A disowned job is not ended; closing the console window ends it, as it ends every process attached to the console."}},
		"enable":    {Summary: "List the builtins.", Operands: "[NAME]...", Options: map[string]string{"a": "every builtin, which is the same as with no option", "p": "the same as with no option", "s": "only the special builtins"}, Notes: []string{"A builtin here cannot be switched off, so -n, -f and -d are refused."}},
		"eval":      {Summary: "Join the arguments and run them as a command.", Operands: "[ARG]..."},
		"exec":      {Summary: "Run a command in place of this shell, or apply redirections to it.", Operands: "[COMMAND [ARG]...]"},
		"exit":      {Summary: "Leave the shell.", Operands: "[N]", Notes: []string{"N is the status, the last command's when it is left out. The EXIT trap runs first."}},
		"logout":    {Summary: "Leave a login shell.", Operands: "[N]", Notes: []string{"In a shell that is no login shell it is an error, and `exit` is what leaves."}},
		"return":    {Summary: "Leave a function or a sourced file.", Operands: "[N]"},
		"export":    {Summary: "Pass variables on to the commands this shell runs.", Operands: "[-n] [-p] [NAME[=VALUE]]...", Notes: []string{"-n stops exporting a name, and -p lists the exported ones."}},
		"readonly":  {Summary: "Make variables readonly, or list them.", Operands: "[-p] [NAME[=VALUE]]..."},
		"unset":     {Summary: "Remove variables or functions.", Operands: "[-fnv] NAME...", Notes: []string{"-f names functions, -v variables, and -n a name reference itself rather than what it names."}},
		"fc":        {Summary: "List, edit and rerun commands from the history.", Operands: "[FIRST [LAST]] | -s [OLD=NEW] [COMMAND]", Options: map[string]string{"e": "the editor to edit them in", "l": "list them rather than edit them", "n": "list them without numbers", "r": "in reverse order", "s": "rerun one, after replacing OLD with NEW"}},
		"getopts":   {Summary: "Read the next option from the positional parameters.", Operands: "OPTSTRING NAME [ARG]..."},
		"hash":      {Summary: "Remember or forget where commands are, or list them.", Operands: "[NAME]...", Options: map[string]string{"r": "forget them all"}},
		"help":      {Summary: "List the builtins and applets, or describe some.", Operands: "[NAME]..."},
		"history":   {Summary: "List or change the command history.", Operands: "[N]", Options: map[string]string{"a": "append this session's new lines to the history file", "c": "clear the history", "d": "delete an entry, a negative one from the end, or a range START-END", "n": "read the lines the file has gained since", "p": "expand the arguments as history and print them", "r": "read the history file into the history", "s": "add the arguments as one entry", "w": "write the history to the history file"}, Notes: []string{"HISTFILE names the file, ~/.nemosh_history by default; HISTSIZE, HISTFILESIZE and HISTCONTROL are read as bash reads them."}},
		"jobs":      {Summary: "List the background jobs.", Operands: "[-lprs] [JOB]...", Notes: []string{"-l adds the pid, -p prints only the pids, -r only the running jobs and -s only the stopped ones, of which there are none."}},
		"kill":      {Summary: "Send a signal to processes or jobs.", Operands: "[-s SIGNAL | -SIGNAL] PID|JOB... | -l [N]", Notes: []string{"A job is %N. Nothing here can stop a process, so STOP, CONT and TSTP are refused."}},
		"wait":      {Summary: "Wait for jobs to end, and answer the status of the last.", Operands: "[-n] [-p VAR] [PID|JOB]..."},
		"let":       {Summary: "Evaluate arithmetic expressions.", Operands: "EXPRESSION...", Notes: []string{"The status is 1 when the last one is 0, and 0 otherwise."}},
		"mapfile":   {Summary: "Read lines into an array.", Operands: "[-d DELIM] [-n COUNT] [-O ORIGIN] [-s COUNT] [-u FD] [ARRAY]", Options: map[string]string{"t": "drop the delimiter from each line"}, Notes: []string{"ARRAY is MAPFILE when it is left out."}},
		"readarray": {Summary: "Read lines into an array; the same as mapfile.", Operands: "[ARRAY]", Options: map[string]string{"t": "drop the delimiter from each line"}},
		"read":      {Summary: "Read a line and split it into variables.", Operands: "[-r] [-a ARRAY] [-d DELIM] [-n N] [-p PROMPT] [-s] [-t SECONDS] [-u FD] [NAME]...", Notes: []string{"-r takes backslashes as they are; without NAME the line goes to REPLY."}},
		"set":       {Summary: "Set the shell's options and the positional parameters, or list the variables.", Operands: "[-+OPTIONS] [-+o NAME]... [--] [ARG]...", Notes: []string{"`set -o` lists the options by name. -b, -m and -v are refused, with the reason."}},
		"shift":     {Summary: "Move the positional parameters down by N, one by default.", Operands: "[N]"},
		"shopt":     {Summary: "Set or list bash's optional behaviours.", Operands: "[-pqsu] [-o] [NAME]...", Notes: []string{"Every bash name is known; those that cannot act here say so."}},
		"time":      {Summary: "Run a command and say how long it took.", Operands: "[-p] COMMAND [ARG]..."},
		"timeout":   {Summary: "Run a command and end it if it runs too long.", Operands: "[-s SIGNAL] SECONDS COMMAND [ARG]..."},
		"times":     {Summary: "Print the time this shell and its children have used."},
		"trap":      {Summary: "Run a command when a signal arrives or the shell exits.", Operands: "[-lp] [[ACTION] SIGNAL...]", Notes: []string{"An empty ACTION ignores the signal, and `-` puts it back. EXIT, ERR, DEBUG and RETURN are taken as bash takes them."}},
		"type":      {Summary: "Say what each name is: an alias, a keyword, a function, a builtin or a file.", Operands: "NAME...", Options: map[string]string{"a": "every way the name can be read, not only the first", "f": "pass over functions", "p": "only the path of a file", "P": "the path of a file, even when the name is something else first", "t": "only the kind, as one word"}},
		"umask":     {Summary: "Set or show the file mode mask.", Operands: "[-p] [-S] [MODE]"},
		"which":     {Summary: "Say where on PATH each command is.", Operands: "[-a] NAME..."},
		"flock":     {Summary: "Run a command holding a lock on a file.", Operands: "FILE COMMAND [ARG]...", Options: map[string]string{"s": "a shared lock", "x": "an exclusive lock, which is the default", "u": "release the lock", "n": "fail rather than wait", "shared": "the same as -s", "exclusive": "the same as -x", "unlock": "the same as -u", "nonblock": "the same as -n"}},
		"watch":     {Summary: "Run a command every few seconds, showing its output.", Operands: "COMMAND [ARG]...", Options: map[string]string{"d": "mark what changed since the last run", "n": "the seconds between runs, 2 by default", "t": "leave out the header", "x": "run the arguments as they are rather than through the shell"}},
	}
}
