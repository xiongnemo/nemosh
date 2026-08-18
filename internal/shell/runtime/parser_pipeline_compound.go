package runtime

import (
	"errors"
	"strings"
)

// A compound as a pipeline stage: `cmd | while read -r line; do ...; done`.
//
// It reported `unexpected do`, because the span builder finds a compound only at the
// start of a line and `printf x | while read -r l` does not start with one. That is the
// canonical way to process a command's output line by line, so between this and
// `done < file` -- fixed in the previous round -- there was no direct spelling for
// reading input into a loop at all.
//
// The compound becomes a brace group, which is what it already becomes when a
// redirection follows its closer, and a brace group has been usable as a pipeline stage
// all along. So the work here is entirely in *finding* it: the words before the pipe are
// parsed as the pipeline they are, and the group is appended as its last stage.

// errMissingPipelineStage is what a prefix that parsed to nothing gets. It should not be
// reachable -- splitPipelineCompound refuses an empty prefix -- and saying so is cheaper
// than a nil dereference if it ever is.
var errMissingPipelineStage = errors.New("syntax error: missing command before |")

// compoundKeywords are the words that begin a compound. `then`, `do`, `else` and the
// closers are not among them: those continue a compound rather than open one, and a
// pipe into `then` is not a thing.
var compoundKeywords = [...]string{"if", "while", "until", "for", "case"}

// splitPipelineCompound finds a compound that begins after a pipe, and returns the words
// before the pipe and the compound's own header.
//
// The *last* such pipe, so `a | b | while ...` puts both `a` and `b` in the prefix. The
// prefix keeps its own pipes and is parsed as an ordinary line, which is what makes an
// and-or in front of it work too: `x && a | while ...` is `x && (a | while ...)`, and
// parsing `x && a` gives exactly that shape to append to.
func splitPipelineCompound(line string) (string, string, bool) {
	prefix, rest, found := "", "", false
	for _, index := range topLevelPipes(line) {
		candidate := strings.TrimLeft(line[index+1:], " \t")
		if !beginsWithCompoundKeyword(candidate) {
			continue
		}
		before := strings.TrimSpace(line[:index])
		if before == "" {
			// `| while ...` with nothing in front of it is a syntax error, and calling
			// it a pipeline stage here would hide that.
			continue
		}
		prefix, rest, found = before, candidate, true
	}
	return prefix, rest, found
}

// topLevelPipes reports the offsets of the `|` characters that separate pipeline stages:
// outside quotes, outside any bracket, and not part of `||`.
func topLevelPipes(line string) []int {
	var offsets []int
	quote := byte(0)
	depth := 0
	escaped := false
	for index := 0; index < len(line); index++ {
		char := line[index]
		switch {
		case escaped:
			escaped = false
		case char == '\\' && quote != '\'':
			escaped = true
		case quote != 0:
			if char == quote {
				quote = 0
			}
		case char == '\'' || char == '"':
			quote = char
		case char == '(' || char == '{' || char == '[':
			depth++
		case char == ')' || char == '}' || char == ']':
			if depth > 0 {
				depth--
			}
		case char == '|' && depth == 0:
			if index+1 < len(line) && line[index+1] == '|' {
				index++
				continue
			}
			if index > 0 && line[index-1] == '|' {
				continue
			}
			offsets = append(offsets, index)
		}
	}
	return offsets
}

func beginsWithCompoundKeyword(text string) bool {
	for _, keyword := range compoundKeywords {
		rest, ok := strings.CutPrefix(text, keyword)
		if !ok {
			continue
		}
		// The keyword has to be a whole word: `iffy | ...` is a command, and `while`
		// with nothing after it is not a header either.
		if rest != "" && (rest[0] == ' ' || rest[0] == '\t' || rest[0] == '(') {
			return true
		}
	}
	return false
}

// wrapCompoundIntoPipeline makes the compound the last stage of the pipeline the words
// before it describe.
//
// The compound goes in as a brace group, which is how a redirection after its closer is
// already handled -- one representation for "a compound used where a command is
// expected", so the two cannot drift apart. In bash a loop in a pipeline runs in a
// subshell and so does a brace group in one, which is why the scoping needs no
// separate thought: a variable set inside `cmd | while read` does not survive either
// way.
func wrapCompoundIntoPipeline(node programNode, prefix string, budget *parseBudget, depth int) (programNode, error) {
	prior, err := parseTypedLineWithBudget(prefix, budget, depth)
	if err != nil {
		return nil, err
	}
	if len(prior.items) == 0 {
		return nil, errMissingPipelineStage
	}
	group := braceGroup{body: Script{program: []programNode{node}}}
	// The last pipeline of the last and-or item is the one the pipe belonged to:
	// `x && a | while ...` pipes into the `a` stage, not into `x`.
	item := len(prior.items) - 1
	andor := prior.items[item].value
	if len(andor.pipelines) == 0 {
		return nil, errMissingPipelineStage
	}
	stage := len(andor.pipelines) - 1
	andor.pipelines[stage].commands = append(andor.pipelines[stage].commands, group)
	prior.items[item].value = andor
	return listNode{value: prior}, nil
}

// splitCloserOperator reads a suffix that begins with a pipe or an and-or operator,
// returning the operator and the words after it.
func splitCloserOperator(suffix string) (string, string, bool) {
	for _, operator := range [...]string{"&&", "||", "|"} {
		rest, ok := strings.CutPrefix(suffix, operator)
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		if rest == "" {
			// `done |` with nothing after it is a syntax error, and the pipeline parser
			// says so better than this could.
			return "", "", false
		}
		return operator, rest, true
	}
	return "", "", false
}

// wrapCompoundBeforeOperator makes the compound the first term of what follows it:
// `done | cat` is a pipeline whose first stage is the compound, and `esac && echo` is an
// and-or whose first term is.
//
// The words after the operator are parsed as an ordinary line and the compound is put in
// front, which is why `done | cat | wc -l` and `esac && a || b` need no separate
// handling -- the parse of the remainder already has the right shape.
func wrapCompoundBeforeOperator(node programNode, operator, rest string, budget *parseBudget, depth int) (programNode, error) {
	following, err := parseTypedLineWithBudget(rest, budget, depth)
	if err != nil {
		return nil, err
	}
	if len(following.items) == 0 || len(following.items[0].value.pipelines) == 0 {
		return nil, errMissingPipelineStage
	}
	group := braceGroup{body: Script{program: []programNode{node}}}
	andor := following.items[0].value
	if operator == "|" {
		// One pipeline, with the compound as its first stage.
		andor.pipelines[0].commands = append([]commandNode{group}, andor.pipelines[0].commands...)
		following.items[0].value = andor
		return listNode{value: following}, nil
	}
	// An and-or: the compound is a pipeline of its own in front of the rest, and the
	// operator joins them.
	kind := tokenAndIf
	if operator == "||" {
		kind = tokenOrIf
	}
	andor.pipelines = append([]pipeline{{commands: []commandNode{group}}}, andor.pipelines...)
	andor.operators = append([]tokenKind{kind}, andor.operators...)
	following.items[0].value = andor
	return listNode{value: following}, nil
}
