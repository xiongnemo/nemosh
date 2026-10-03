package runtime

import (
	"fmt"
	"strings"
)

// The layered error contract of docs/design/v0-scope.md P1.1: one stable first
// line, an optional hint that says what to do instead, and detail that appears
// only when the reader asked for it.
//
// The layering exists because the three have different audiences. A script
// greps the first line, so it must not move. A person needs the hint, and a
// hint printed every time is noise the person stops reading. The detail is for
// whoever is debugging the shell itself, and printing it by default would leak
// host paths into output that behavior cases compare byte for byte.
type shellDiagnostic struct {
	// message is the first line, after diagnosticPrefix and the command's name.
	message string
	// hint names the way out. Omitted when there is nothing useful to say;
	// "try again" is not a hint.
	hint string
	// channel gates the details. Empty means there are none.
	channel debugChannel
	details []string
}

// debugChannel names an opt-in stream of detail, selected by NEMOSH_DEBUG.
type debugChannel string

const (
	// debugPath covers how an operand became a host path: the aliases tried,
	// the current root, the spelling that came back.
	debugPath debugChannel = "path"
	// debugExec covers lookup and launch: the directories searched, the
	// suffixes tried, the command line handed to Windows.
	debugExec debugChannel = "exec"
	// debugFD covers the descriptor table: what was opened, bound, and closed.
	debugFD debugChannel = "fd"
	// debugPanic covers a recovered panic's stack trace. Off by default because
	// the trace carries host paths and the behavior corpus compares output byte
	// for byte; the diagnostic itself always prints and names this channel.
	debugPanic debugChannel = "panic"
)

var knownDebugChannels = []debugChannel{debugPath, debugExec, debugFD, debugPanic}

// diagnosticPrefix is what the shell's own messages begin with, as bash's error_prolog
// writes it: the file the running code is in, or $0 where there is none, and the running
// line -- `build.sh: line 3: `, and `nemosh: line 1: ` for a command string. At a prompt it
// is the name alone, the line being the one just typed. It was `nemosh: ` everywhere, so a
// script's message did not say which script, or where in it.
func (r Runtime) diagnosticPrefix() string {
	if r.interactive.session {
		return r.diagnosticName() + ": "
	}
	return fmt.Sprintf("%s: line %d: ", r.diagnosticName(), r.currentLine())
}

// diagnosticName is bash's get_name_for_error: $BASH_SOURCE's first element, then $0, then
// the shell's own name; and the shell's name at a prompt.
func (r Runtime) diagnosticName() string {
	if r.interactive.session {
		return "nemosh"
	}
	if file := r.currentFile(); file != "" {
		return file
	}
	if r.params != nil && r.params.name != "" {
		return r.params.name
	}
	return "nemosh"
}

// report writes a diagnostic in its layers, after diagnosticPrefix. The prefix is the
// command's name where there is one and `nemosh` where the shell itself is speaking.
func (r Runtime) report(prefix string, diagnostic shellDiagnostic) {
	if prefix == "nemosh" {
		prefix = ""
	} else {
		prefix += ": "
	}
	fmt.Fprintf(r.streams.Stderr, "%s%s%s\n", r.diagnosticPrefix(), prefix, diagnostic.message)
	if diagnostic.hint != "" {
		fmt.Fprintf(r.streams.Stderr, "hint: %s\n", diagnostic.hint)
	}
	if diagnostic.channel == "" || !r.debugEnabled(diagnostic.channel) {
		return
	}
	for _, detail := range diagnostic.details {
		fmt.Fprintf(r.streams.Stderr, "debug: %s: %s\n", diagnostic.channel, detail)
	}
}

// debugEnabled reads NEMOSH_DEBUG, a comma-separated list of channel names.
// `all` turns every channel on.
//
// An unrecognised name is reported once rather than ignored: a misspelled
// channel that silently produced nothing looks exactly like a shell that had
// nothing to say, which is the failure this whole contract exists to avoid.
func (r Runtime) debugEnabled(channel debugChannel) bool {
	setting, ok := r.vars["NEMOSH_DEBUG"]
	if !ok || setting == "" {
		return false
	}
	enabled := false
	for _, name := range strings.Split(setting, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if name == "all" || debugChannel(name) == channel {
			enabled = true
		}
		if !knownDebugChannel(name) {
			r.warnUnknownDebugChannel(name)
		}
	}
	return enabled
}

func knownDebugChannel(name string) bool {
	if name == "all" {
		return true
	}
	for _, channel := range knownDebugChannels {
		if debugChannel(name) == channel {
			return true
		}
	}
	return false
}

// warnUnknownDebugChannel complains once per shell. The set of names is
// checked every time a diagnostic is written, and repeating the complaint would
// bury the diagnostics it is attached to.
func (r Runtime) warnUnknownDebugChannel(name string) {
	if r.expansion == nil || r.expansion.warnedDebugChannels == nil {
		return
	}
	if r.expansion.warnedDebugChannels[name] {
		return
	}
	r.expansion.warnedDebugChannels[name] = true
	fmt.Fprintf(r.streams.Stderr, "%sNEMOSH_DEBUG: unknown channel %q; known channels are path, exec, fd, panic, all\n", r.diagnosticPrefix(), name)
}

// debugDetails builds the detail lines only when the channel is on, so a
// diagnostic never pays for detail nobody will read.
func (r Runtime) debugDetails(channel debugChannel, build func() []string) []string {
	if !r.debugEnabled(channel) {
		return nil
	}
	return build()
}
