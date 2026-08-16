package main

import (
	"sort"
	"strings"

	"github.com/xiongnemo/nemosh/internal/completionspec"
)

// operandTarget is what the word being typed is, once the words before it are
// taken into account.
//
// `ssh ` wants a host and `ssh -i ` wants a file, and the difference is not in
// the word being typed -- it is in the one before it. That is the knowledge zsh
// keeps in a per-command argument grammar and bash-completion hand-writes as a
// `case $prev in` per command; here it comes out of the same capability table
// everything else reads.
type operandTarget int

const (
	targetPath operandTarget = iota
	targetHost
	// targetUnknown is an option's argument that is not a path and not a host:
	// `ssh -p `, a port. Nothing here can guess a port, and offering the files
	// in the current directory would be an answer from the wrong universe. So
	// this offers nothing, which at least reads as "I do not know".
	targetUnknown
	// targetSubcommand is the word that selects a surface: `adb install`.
	targetSubcommand
)

// operandTargetFor decides from the text before the word being typed.
//
// It reads rather than parses, like everything else on this path: the line is
// half-typed and will usually not parse at all.
func operandTargetFor(specs *completionspec.Registry, prefix string) operandTarget {
	words := strings.Fields(commandSegment(prefix))
	surface, ok := surfaceFor(specs, words)
	if !ok {
		return targetPath
	}
	operands := 0
	for index := 1; index < len(words); index++ {
		flag, takesValue := detachedValueOption(surface, words[index])
		if takesValue {
			if index+1 == len(words) {
				// The word being typed is this option's value.
				if surface.takesFile(flag) {
					return targetPath
				}
				return targetUnknown
			}
			// Otherwise the value is the next word and is nobody's operand --
			// which is the whole point of tracking this. Without it `ssh -i key `
			// counted `key` as the host and stopped offering one.
			index++
			continue
		}
		if !strings.HasPrefix(words[index], "-") {
			operands++
		}
	}
	// A command with subcommands wants one of those for its first operand:
	// `adb ins` should reach `install`, not the files in this directory.
	if operands == 0 && len(surface.subcommands) > 0 {
		return targetSubcommand
	}
	// A surface that takes no operand says so, and the honest answer for the
	// word after it is nothing at all -- `adb devices ` takes nothing, and the
	// files in this directory are not what was wanted.
	if surface.operand == completionspec.OperandNone {
		return targetUnknown
	}
	if surface.operand != completionspec.OperandHost {
		return targetPath
	}
	// Only the first operand is a host. `ssh host command...` runs *command* on
	// the far side, and nothing here can know what lives there -- bash-completion
	// answers it by opening a real connection to the remote machine, which is a
	// round trip this shell will not make behind someone's back.
	if operands > 0 {
		return targetUnknown
	}
	return targetHost
}

// detachedValueOption reports whether the word is an option whose value is the
// *next* word, and which option it is.
//
// Only the detached spelling counts. `ssh -i key` has the value in the following
// word; `ssh -ikey` carries it already, so what comes after that is an operand
// again. The length is enough to tell them apart.
func detachedValueOption(surface commandSurface, word string) (rune, bool) {
	if len(word) != 2 || word[0] != '-' {
		return 0, false
	}
	flag := rune(word[1])
	return flag, surface.takesValue(flag)
}

// commandSegment is the current command and the words already typed after it,
// dropping everything up to the last separator that begins a new command.
func commandSegment(prefix string) string {
	if start := strings.LastIndexAny(prefix, "|&;({"); start >= 0 {
		return prefix[start+1:]
	}
	return prefix
}

// completeHost offers host names for the word being typed.
//
// `user@host` is split and rejoined: the part before the `@` is a login name,
// which this shell has no list of, and completing the host after it is what was
// wanted. fish does the same through __fish_complete_user_at_hosts.
//
// There is no fallback to paths when nothing matches. Everywhere else a failed
// specific completion falls back to the ordinary one -- that is what keeps a file
// named `-1.18-windows.xml` reachable -- but a host is not a path, and answering
// `ssh nonexist<TAB>` with `notes.txt` would be the wrong kind rather than a
// wider guess. Nothing to offer rings the bell, which is the honest answer.
func completeHost(hosts []string, stem string) []string {
	login := ""
	name := stem
	if at := strings.LastIndexByte(stem, '@'); at >= 0 {
		login, name = stem[:at+1], stem[at+1:]
	}
	var matches []string
	for _, host := range hosts {
		if completionMatches(host, name) {
			matches = append(matches, login+host)
		}
	}
	sortCandidates(matches)
	return slicesCompact(matches)
}

// completeOperandWord answers for a word that is not the command name.
//
// The order is: an option if the word asks for one, then whatever kind of
// operand the words before it imply. Options come first whatever the kind,
// because a leading dash is a request for an option and `ssh -` should not be
// answered with a host called `-something`.
func (e *lineEditor) completeOperandWord(prefix, stem string) ([]string, bool) {
	words := strings.Fields(commandSegment(prefix))
	surface, known := surfaceFor(e.specs, words)
	// An option is asked for by the dash, whatever kind the operand is, and the
	// surface decides which options exist -- `adb install -` and `adb -` are not
	// the same question.
	if known && strings.HasPrefix(stem, "-") {
		if offers := matchingOptions(surface, stem); len(offers) > 0 {
			return offers, true
		}
	}
	switch operandTargetFor(e.specs, prefix) {
	case targetHost:
		return completeHost(e.hosts.candidates(), stem), false
	case targetSubcommand:
		return matchingSubcommands(surface, stem), false
	case targetUnknown:
		return nil, false
	}
	return completeOperand(e.workingDirectory, commandInProgress(prefix), stem)
}

func matchingOptions(surface commandSurface, stem string) []string {
	var matches []string
	for _, option := range surface.options() {
		if strings.HasPrefix(option, stem) {
			matches = append(matches, option)
		}
	}
	sort.Strings(matches)
	return matches
}

func matchingSubcommands(surface commandSurface, stem string) []string {
	var matches []string
	for _, name := range surface.subcommands {
		if completionMatches(name, stem) {
			matches = append(matches, name)
		}
	}
	sortCandidates(matches)
	return slicesCompact(matches)
}
