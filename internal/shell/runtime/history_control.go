package runtime

import (
	"strconv"
	"strings"
)

// `HISTCONTROL` and `HISTSIZE`, and what `$!` can honestly be here.
//
// **HISTCONTROL is a privacy feature that people assume works.** A leading space is how
// everyone keeps a token out of their history -- ` export GITHUB_TOKEN=...` -- and both
// variables were settable and had no effect whatever, so the habit silently did nothing
// and the secret went into the file on disk. That is worse than not offering it: a
// refusal would at least have been noticed.

// historyControl is the parsed HISTCONTROL value.
type historyControl struct {
	ignoreSpace bool
	ignoreDups  bool
	eraseDups   bool
}

func parseHistoryControl(value string) historyControl {
	control := historyControl{}
	for _, field := range strings.Split(value, ":") {
		switch strings.TrimSpace(field) {
		case "ignorespace":
			control.ignoreSpace = true
		case "ignoredups":
			control.ignoreDups = true
		case "ignoreboth":
			control.ignoreSpace, control.ignoreDups = true, true
		case "erasedups":
			control.eraseDups = true
		}
		// An unknown word is ignored rather than refused, which is bash's behaviour
		// and the safe direction here: refusing would make a shared rc file that
		// names a bash-only setting fail to start a session.
	}
	return control
}

// historyLimit is HISTSIZE, or the built-in ceiling when it is unset or unusable.
//
// A negative or non-numeric value falls back rather than refusing, for the same reason:
// an rc file is shared between shells and must not be able to stop this one starting.
// Zero is honoured, and means keep nothing -- which is a real way to run.
func (r Runtime) historyLimit() int {
	value, ok := r.vars["HISTSIZE"]
	if !ok {
		return maxHistoryEntries
	}
	size, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || size < 0 {
		return maxHistoryEntries
	}
	if size > maxHistoryEntries {
		return maxHistoryEntries
	}
	return size
}

// recordHistoryLine applies HISTCONTROL and HISTSIZE, then records.
//
// The *raw* line is what the space rule reads, so this has to run before anything trims
// it. That is the whole subtlety: by the time a line has been through the parser there is
// no leading space left to notice.
func (r Runtime) recordHistoryLine(raw string) {
	control := parseHistoryControl(r.vars["HISTCONTROL"])
	if control.ignoreSpace && strings.HasPrefix(raw, " ") {
		return
	}
	line := strings.TrimRight(raw, "\n")
	if strings.TrimSpace(line) == "" {
		return
	}
	if control.ignoreDups {
		if entries := r.history.list(); len(entries) > 0 && entries[len(entries)-1] == line {
			return
		}
	}
	if control.eraseDups {
		r.history.erase(line)
	}
	r.history.record(line)
	r.history.truncate(r.historyLimit())
}

// RecordInteractiveLine is the interactive loops' entry point, which applies HISTCONTROL.
//
// Separate from RecordHistory, which the startup file's replay uses: lines read back from
// disk have already been filtered once, and running them through the space rule again
// would drop nothing while costing a parse of HISTCONTROL per line.
func (r Runtime) RecordInteractiveLine(raw string) { r.recordHistoryLine(raw) }
