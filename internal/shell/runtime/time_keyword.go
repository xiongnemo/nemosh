package runtime

import (
	"context"
	"strings"
)

// bash's `time` keyword, over a compound command:
//
//	time { make; make test; }
//	time ( cd build && ninja )
//	time -p for f in *.c; do cc -c "$f"; done
//
// busybox's time is a command (time_builtin.go), so it can only time a command, and each of these
// is a syntax error there, which is why bash decides them. A `time` before a simple command is
// still busybox's command, `time [-pa] [-f FMT] [-o FILE] PROG ARGS`; the keyword takes the
// compound and the rest of its pipeline, as bash's takes the whole pipeline, and reports in the
// same form, $TIME's or busybox's, `-p` POSIX's. It was "unexpected }", and `time ( ... )` read
// as a function named time.

const (
	timeKeyword      = "time"
	timeKeywordPOSIX = "time -p"
)

// afterTimeKeyword reports a brace after `time` or `time -p` where a command begins, which
// opens the group the keyword times. Only there: in `echo time {` the brace is a word.
func afterTimeKeyword(line string, index int) bool {
	prefix := line[:index]
	for offset := len(prefix) - 1; offset >= 0; offset-- {
		if isCommandSeparator(prefix[offset]) {
			prefix = prefix[offset+1:]
			break
		}
	}
	fields := strings.Fields(prefix)
	keyword := len(fields) - 1
	if keyword > 0 && fields[keyword] == "-p" {
		keyword--
	}
	if keyword < 0 || fields[keyword] != "time" {
		return false
	}
	return keyword == 0 || commandIntroducers[fields[keyword-1]]
}

// stripPipelineTime takes `time` or `time -p` off a pipeline whose first command is a group, and
// says which form it was. Before anything else it is no keyword, and stays the command.
func stripPipelineTime(tokens []shellToken) ([]shellToken, string) {
	if len(tokens) < 2 || !isUnquotedWordToken(tokens[0], "time") {
		return tokens, ""
	}
	rest, form := tokens[1:], timeKeyword
	if len(rest) > 1 && isUnquotedWordToken(rest[0], "-p") {
		rest, form = rest[1:], timeKeywordPOSIX
	}
	if rest[0].group == nil {
		return tokens, ""
	}
	return rest, form
}

// isUnquotedWordToken reports a word written as text, unquoted, the way a reserved word is.
func isUnquotedWordToken(token shellToken, text string) bool {
	return token.kind == tokenWord && token.value == text && token.parsed != nil && isUnquotedLiteralWord(*token.parsed)
}

// timedCompoundHeader is the compound after `time` or `time -p` at the front of a line, for the
// span builder, and the form, which comes back as the operator: `time while read l; do`.
func timedCompoundHeader(line string) (string, string, bool) {
	rest, ok := strings.CutPrefix(line, "time ")
	if !ok {
		return "", "", false
	}
	rest, form := strings.TrimLeft(rest, " \t"), timeKeyword
	if after, posix := strings.CutPrefix(rest, "-p "); posix {
		rest, form = strings.TrimLeft(after, " \t"), timeKeywordPOSIX
	}
	if !beginsWithCompoundKeyword(rest) {
		return "", "", false
	}
	return form, rest, true
}

// timedCompound is a compound the keyword times: a brace group around it, as the one pipeline
// of a timed list, so the time runs over the compound and whatever is joined to it.
func timedCompound(node programNode, form string) programNode {
	group := braceGroup{body: Script{program: []programNode{node}}}
	stage := pipeline{commands: []commandNode{group}, timed: form}
	return listNode{value: listOf(listItem{value: andOr{pipelines: []pipeline{stage}}})}
}

// runTimed runs a timed pipeline and reports what it used on stderr, as time does.
func (r Runtime) runTimed(form string, run func() lineResult) lineResult {
	var result lineResult
	usage := r.measure(func() int {
		result = run()
		return result.status
	})
	writeTimeReport(r.streams.Stderr, r.timeFormat(form == timeKeywordPOSIX), []string{"time"}, usage)
	return result
}

// executeTimedPipeline is executeTypedPipeline for a pipeline the keyword times.
func (r Runtime) executeTimedPipeline(ctx context.Context, value pipeline, savedStatus int) lineResult {
	form := value.timed
	value.timed = ""
	return r.runTimed(form, func() lineResult { return r.executeTypedPipeline(ctx, value, savedStatus) })
}
