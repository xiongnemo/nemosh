package runtime

import (
	"fmt"
	"strconv"
)

// How pushd, popd and dirs read their words, as bash's pushd.def does; busybox has none of
// the three. `--` ends the options and `-n` moves no directory. A word that starts with `+`
// or `-` names an entry, and one that is not a number after its sign is an invalid number --
// `-z` included, which bash never reads as an option -- said with the usage line, status 2.
// So is any other word popd is given, as an invalid argument, and any word dirs does not
// know, as an invalid option: dirs takes one letter a word. `pushd -- dir` and `popd --`
// were refused, `-n` was not known, and every misuse was status 1.

var dirStackUsage = map[string]string{
	"pushd": "pushd [-n] [+N | -N | dir]",
	"popd":  "popd [-n] [+N | -N]",
	"dirs":  "dirs [-clpv] [+N] [-N]",
}

// misuseDirStack says a word the builtin cannot read, then its usage, and answers 2.
func (r Runtime) misuseDirStack(builtin, word, problem string) int {
	fmt.Fprintf(r.streams.Stderr, "%s: %s: %s\n", builtin, word, problem)
	fmt.Fprintf(r.streams.Stderr, "%s: usage: %s\n", builtin, dirStackUsage[builtin])
	return 2
}

// stackPosition reads a `+N` or `-N` word as the entry it names: +0 is the current directory
// and -0 the oldest entry. ok is false for a word that is not a number after its sign, and
// inRange for one past the end of the stack.
func (r Runtime) stackPosition(word string) (position int, ok, inRange bool) {
	if !isDigits(word[1:]) {
		return 0, false, false
	}
	number, err := strconv.Atoi(word[1:])
	depth := len(r.dirStack.below)
	if err != nil || number > depth {
		return 0, true, false
	}
	if word[0] == '-' {
		number = depth - number
	}
	return number, true, true
}

// stackOutOfRange is bash's pushd_error: on an empty stack that is what is said, and
// otherwise that the word is out of range.
func (r Runtime) stackOutOfRange(builtin, word string) int {
	if len(r.dirStack.below) == 0 {
		fmt.Fprintf(r.streams.Stderr, "%s: directory stack empty\n", builtin)
	} else {
		fmt.Fprintf(r.streams.Stderr, "%s: %s: directory stack index out of range\n", builtin, word)
	}
	return 1
}

// isStackWord reports a word that names an entry, or tries to.
func isStackWord(word string) bool {
	return word != "" && (word[0] == '+' || word[0] == '-')
}

// pushd puts the current directory on the stack and changes to another; `+N` turns the
// stack instead. After `-n` nothing moves: a directory is only put on the stack, as the word
// it was given.
func (r Runtime) pushd(args []string) int {
	words, reading := args, true
	if len(words) > 0 && words[0] == "--" {
		words, reading = words[1:], false
	}
	if len(words) == 0 {
		return r.exchangeTopDirectories()
	}
	noChange, rotation := false, -1
	for reading && len(words) > 0 {
		word := words[0]
		if word == "--" {
			words = words[1:]
			break
		}
		if word == "-" || !isStackWord(word) {
			break
		}
		if word == "-n" {
			noChange = true
		} else if position, ok, inRange := r.stackPosition(word); !ok {
			return r.misuseDirStack("pushd", word, "invalid number")
		} else if !inRange {
			return r.stackOutOfRange("pushd", word)
		} else {
			rotation = position
		}
		words = words[1:]
	}
	switch {
	case rotation >= 0:
		return r.rotateDirectoryStack(rotation, noChange)
	case len(words) == 0:
		return 0
	case len(words) > 1:
		fmt.Fprintln(r.streams.Stderr, "pushd: too many arguments")
		return 1
	case noChange:
		r.dirStack.below = append([]string{words[0]}, r.dirStack.below...)
	default:
		previous := r.WorkingDirectory()
		if status := r.changeDirectory("pushd", []string{"--", words[0]}); status != 0 {
			return status
		}
		r.dirStack.below = append([]string{previous}, r.dirStack.below...)
	}
	return r.printDirectoryStack(dirsFormat{})
}

// popd removes an entry, the top one unless a `+N` or `-N` names another, changing to the
// one beneath when it removes the current directory -- unless `-n` says not to move.
func (r Runtime) popd(args []string) int {
	noChange, word, position := false, "", 0
words:
	for _, arg := range args {
		switch {
		case arg == "--" || arg == "":
			break words
		case arg == "-n":
			noChange = true
		case isStackWord(arg):
			found, ok, inRange := r.stackPosition(arg)
			if !ok {
				return r.misuseDirStack("popd", arg, "invalid number")
			}
			if !inRange {
				return r.stackOutOfRange("popd", arg)
			}
			word, position = arg, found
		default:
			return r.misuseDirStack("popd", arg, "invalid argument")
		}
	}
	if len(r.dirStack.below) == 0 {
		return r.stackOutOfRange("popd", word)
	}
	if position > 0 {
		// Anything but the top is removed without moving: the shell stays where it is.
		r.dirStack.below = append(r.dirStack.below[:position-1], r.dirStack.below[position:]...)
		return r.printDirectoryStack(dirsFormat{})
	}
	if !noChange {
		if status := r.changeDirectory("popd", []string{"--", r.dirStack.below[0]}); status != 0 {
			return status
		}
	}
	r.dirStack.below = r.dirStack.below[1:]
	return r.printDirectoryStack(dirsFormat{})
}

// dirs prints the stack, or with `+N` or `-N` the one entry it names -- the last such word
// given, as in bash.
func (r Runtime) dirs(args []string) int {
	format, clear := dirsFormat{}, false
	word, position, inRange := "", 0, true
words:
	for _, arg := range args {
		switch {
		case arg == "-l":
			format.long = true
		case arg == "-c":
			clear = true
		case arg == "-v":
			format.numbered, format.perLine = true, true
		case arg == "-p":
			format.perLine = true
		case arg == "--":
			break words
		case isStackWord(arg):
			found, ok, within := r.stackPosition(arg)
			if !ok {
				return r.misuseDirStack("dirs", arg, "invalid number")
			}
			word, position, inRange = arg[1:], found, within
		default:
			return r.misuseDirStack("dirs", arg, "invalid option")
		}
	}
	if clear {
		r.dirStack.below = nil
		return 0
	}
	if word == "" {
		return r.printDirectoryStack(format)
	}
	if !inRange {
		return r.stackOutOfRange("dirs", word)
	}
	entry := r.abbreviateHome(r.directoryEntries()[position], format.long)
	if format.numbered {
		fmt.Fprintf(r.streams.Stdout, "%2d  %s\n", position, entry)
	} else {
		fmt.Fprintln(r.streams.Stdout, entry)
	}
	return 0
}
