package capability

// The second half of the applet help, split from usage_text.go for the 250-line ceiling
// AGENTS.md sets -- the same reason usage_placeholders.go is its own file.
//
// The seam is chronological rather than alphabetical: these are the entries written for awk
// and for the two tiers of applets that followed it. Splitting by first letter would have
// meant moving unrelated entries every time one was added, and a map that is looked up by
// name does not care which file its keys live in.
//
// The two are **merged into one map** rather than looked up separately, so that everything
// which reads the table -- usage.go and the two tests that hold it to the capability rows --
// sees every entry without knowing there was a split at all.
var usageTextMore = map[string]Usage{
	"dc": {Summary: "A reverse-polish calculator with arbitrary precision.", Operands: "[FILE]...",
		Options: map[string]string{"e": "run this script", "f": "run this file", "x": "accepted; dc reads its input as a script anyway"},
		Notes: []string{
			"A stack machine: `echo \"2 3 + p\" | dc` prints 5. `p` prints the top, `f` the whole stack top first.",
			"`_` is the minus sign of a literal; `-` is subtraction.",
			"k sets the scale, i the input base and o the output base; K, I and O push them.",
			"A register is a whole stack: s replaces it and S pushes onto it, which is what makes recursion expressible. An unset register reads as zero.",
			"An error stops the script, as the reference does -- but output already printed is kept, where the reference loses it.",
			"The shell escape ! is refused: this shell's applets start no processes.",
			"Bases run from 2 to 16. Above that POSIX prints digits as space-separated decimal groups, which is a different format rather than a longer alphabet.",
		}},
	"df": {Summary: "Report free space on each filesystem.", Operands: "[FILE]...",
		Options: map[string]string{"h": "print sizes as K, M and G", "k": "print 1K blocks, which is the default"},
		Notes: []string{
			"A filesystem here is a drive letter, mounted at its root -- C: on C:/ -- because that is what Windows volumes are.",
			"The percentage is of used plus available rather than of the raw size, so a quota does not make a full disk read low.",
			"A drive that is not ready is skipped rather than reported as zero, which would read like a full disk.",
			"Away from Windows it reports the filesystems the operands are on rather than enumerating every mount.",
		}},
	"dd": {Summary: "Copy blocks, converting them.", Operands: "[if=FILE] [of=FILE] [bs=N] [count=N] [skip=N] [seek=N] [conv=LIST] [status=none]",
		Notes: []string{
			"Operands are name=value, not options. An unrecognised one is refused, where busybox prints its usage and exits 0 -- so a typo like cnt=3 copies the whole file and reports success there.",
			"Sizes take the suffixes c (1), w (2), b (512), K, M, G and the decimal KB, MB, GB, and a product form such as 2x512.",
			"conv takes notrunc, sync, ucase, lcase, swab and fsync. conv=noerror is refused: skipping a read failure is a recovery mode this does not implement.",
			"The record counts go to standard error, so dd can still be used in a pipeline. status=none silences them.",
			"A short read is a whole record unless conv=sync pads it, which is why dd bs=1M on a pipe often copies less than expected.",
		}},
	"stty": {Summary: "Report or set terminal settings.", Operands: "[size|echo|-echo|sane]",
		Options: map[string]string{"a": "report everything this can say"},
		Notes: []string{
			"Scoped to what a Windows console can answer: the window size and whether it echoes. The rest of stty describes a serial line, which Windows has no equivalent of.",
			"Anything else is refused by name. An stty that accepted -icanon and did nothing would leave a script believing the terminal had changed.",
			"On Windows, turning echo off also leaves line assembly, because the console echoes as part of assembling a line.",
		}},
	"cal": {Summary: "Print a calendar.", Operands: "[[MONTH] YEAR]",
		Options: map[string]string{"m": "start the week on Monday", "y": "print the whole year"},
		Notes: []string{
			"September 1752 is short: eleven days were dropped for the Gregorian reformation, and every cal since the original prints it that way.",
			"busybox's -j, the day-of-year form, is not here: its own source records that `cal -j 1752` is wrong.",
		}},
	"getopt": {Summary: "Canonicalise a command line for a shell script to read.", Operands: "[OPTSTRING] PARAMS",
		Options: map[string]string{"o": "the short options to recognise", "l": "the long options to recognise, comma separated",
			"n": "the name to report errors under", "q": "no diagnostics", "Q": "no output", "u": "do not quote the output",
			"a": "allow long options with a single dash", "T": "test for the enhanced version, exiting 4", "s": "the shell quoting convention",
			"options": "the short options to recognise", "long": "the long options to recognise", "longoptions": "the long options to recognise",
			"name": "the name to report errors under", "shell": "the shell quoting convention", "quiet": "no diagnostics",
			"quiet-output": "no output", "unquoted": "do not quote the output", "alternative": "allow long options with a single dash",
			"test": "test for the enhanced version, exiting 4"},
		Notes: []string{
			"Meant for `eval set -- \"$(getopt -o ab: -- \"$@\")\"`, which is why the output is quoted.",
			"The old form with no -o -- `getopt ab: \"$@\"` -- prints unquoted, as the original did.",
			"Only the sh and bash quoting conventions are supported; -s csh is refused.",
		}},
	"ipcalc": {Summary: "Work out network numbers from an address.", Operands: "ADDRESS[/PREFIX] [NETMASK]",
		Options: map[string]string{"b": "the broadcast address", "n": "the network address", "m": "the netmask",
			"p": "the prefix length", "h": "the resolved host name", "s": "no diagnostics"},
		Notes: []string{
			"The output is KEY=value lines in a fixed order, so a script can eval it.",
			"A netmask with holes in it is used as given rather than refused: the operation is a bitwise and, which is well defined either way.",
			"An address with no prefix and no netmask falls back to the old class rule, which is what every ipcalc does.",
		}},
	"arch":    {Summary: "Print the machine architecture.", Notes: []string{"The same name `uname -m` gives, from the same mapping."}},
	"logname": {Summary: "Print the login name.", Notes: []string{"The account that logged in, which under elevation still differs from `whoami`: that answers root, this answers the account."}},
	"groups": {Summary: "Print the groups a user is in.", Operands: "[USER]",
		Notes: []string{"Windows has no group in the Unix sense, so this answers the one name `id -gn` gives. A user other than this one cannot be looked up and is refused rather than answered wrongly."}},
	"uuidgen": {Summary: "Print a new unique identifier.", Notes: []string{"Version 4, from a cryptographic source, so it does not repeat between runs."}},
	"nproc": {Summary: "Print how many processors are available.",
		Options: map[string]string{"all": "count them all", "ignore": "hold N back"},
		Notes:   []string{"Windows reports only the processors this process may use, so --all is accepted and answers the same number. The answer never drops below 1."}},
	"usleep": {Summary: "Wait for a number of microseconds.", Operands: "MICROSECONDS"},
	"ts": {Summary: "Stamp each line of input with the time it arrived.", Operands: "[FORMAT]",
		Options: map[string]string{"i": "the gap since the previous line", "s": "the time since the first line"},
		Notes:   []string{"The default format is `%b %e %H:%M:%S`. A FORMAT operand takes the strftime conversions %Y %m %d %b %a %e %H %M %S %Z %F %T; an unknown one is left as written.", "Each line is flushed as it is stamped, so a slow pipeline can be watched rather than waited for."}},
	"truncate": {Summary: "Set a file's size.", Operands: "FILE...",
		Options: map[string]string{"s": "the size", "c": "do not create a missing file"},
		Notes: []string{
			"SIZE may carry K, M, G (1024-based) or KB, MB, GB (1000-based).",
			"A leading + or - makes it relative to the current size; GNU's <, >, / and % forms are refused by name.",
			"-c leaves a missing file alone and is not an error, which is what makes `truncate -c -s 0 maybe.log` safe to run either way.",
		}},
	"link": {Summary: "Make another name for a file.", Operands: "FILE LINK",
		Notes: []string{"A hard link, and an existing LINK is refused rather than replaced."}},
	"unlink": {Summary: "Remove one name.", Operands: "FILE",
		Notes: []string{"`rm` with no options, no recursion and no prompt. A directory is refused: that is what `rmdir` is for."}},
	"pidof": {Summary: "Print the ids of processes with a name.", Operands: "NAME...",
		Options: map[string]string{"s": "print only one", "o": "omit these ids"},
		Notes:   []string{"The name is matched whole, with or without an executable suffix, which is what separates this from `pgrep`: `pidof sh` does not find `bash`.", "Nothing matching is exit 1 with no diagnostic, so `pidof x || start x` reads cleanly."}},
	"killall": {Summary: "Signal every process with a name.", Operands: "[-SIGNAL] NAME...",
		Options: map[string]string{"l": "list the signal names", "q": "no diagnostic when nothing matched"},
		Notes:   []string{"The name is matched whole rather than as a pattern, which is the difference between a tidy-up and an accident.", "Windows has no signals; each name is a behaviour this shell reproduces. See `kill`."}},
	"awk": {Summary: "Scan text for patterns and act on them.", Operands: "PROGRAM [FILE|VAR=VALUE]...",
		Options: map[string]string{"F": "the field separator", "v": "set VAR=VALUE before BEGIN", "f": "read the program from a file"},
		Notes: []string{
			"The POSIX language: patterns and actions, BEGIN and END, fields, arrays, user functions, getline and printf.",
			"length, substr, index, match and split count runes rather than bytes, so length(\"héllo\") is 5.",
			"A command in `print | cmd`, `cmd | getline` or system() must be an applet of this shell: no OS process is started, and shell syntax in one is refused rather than guessed at.",
			"RS is a single character or empty for paragraph mode; a regular expression RS is a gawk extension and is not accepted.",
			"gensub, asort, switch, @include and BEGINFILE/ENDFILE are gawk extensions and are refused by name.",
		}},
}

func init() {
	for name, usage := range usageTextMore {
		usageText[name] = usage
	}
}
