package runtime

import "strings"

// A compound, or a function definition, that begins after an operator on the same line.
//
// POSIX 2.9.3 makes every pipeline of an and-or list a command, and a command can be a
// compound: `[ -n "$x" ] && case $x in ...`, `cmd || for f in ...`, `sleep 1 & while ...`
// are ordinary scripts. They were syntax errors -- `unexpected then`, `unexpected ;;` --
// because the span builder finds a compound only at the start of a line, and after `|` since
// the pipeline form was added; after `&&`, `||`, `&` and `!` it did not look. A brace group
// and a subshell worked there all along, being parsed as commands rather than found as
// spans, which is why the gap was only ever the keyword compounds.
//
// This generalises the pipeline form rather than adding a second one. The line is cut at
// the first top-level operator whose right side opens a compound, the words before it are
// parsed as the line they are, and the compound joins them the way the operator says: as
// the last pipeline stage for `|`, the next term for `&&` and `||`, the next list item after
// a background one for `&`. A leading `!` negates the compound's own pipeline.

// operatorAt is one top-level separator on a line: `|`, `|&`, `||`, `&&` or `&`.
type operatorAt struct {
	offset int
	text   string
}

// topLevelOperators reports the separators outside quotes and brackets, so an `&&` inside
// `[[ ]]` or `$(( ))` is not one. The `&` of a redirection -- `>&2`, `<&3`, `&>file` -- and
// bash's `|&` is one, the pipe that carries stderr too.
func topLevelOperators(line string) []operatorAt {
	var found []operatorAt
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
		case depth == 0 && (char == '|' || char == '&'):
			operator, width := operatorText(line, index)
			if operator != "" {
				found = append(found, operatorAt{offset: index, text: operator})
			}
			index += width - 1
		}
	}
	return found
}

// operatorText names the operator at index, or "" for an `&` or `|` that is not one; the
// width is how much of the line it covers either way.
func operatorText(line string, index int) (string, int) {
	char := line[index]
	next, previous := byte(0), byte(0)
	if index+1 < len(line) {
		next = line[index+1]
	}
	if index > 0 {
		previous = line[index-1]
	}
	switch {
	case next == char:
		return line[index : index+2], 2
	case char == '|' && next == '&':
		return "|&", 2
	case char == '&' && (previous == '>' || previous == '<' || previous == '|' || next == '>'):
		return "", 1
	}
	return string(char), 1
}

// splitCompoundAfterOperator finds a compound that begins after an operator, and returns the
// words before the operator, the operator, and the compound's own header.
//
// The *first* such operator: in `true && while a || b`, the `||` is the loop's condition,
// not a separator in front of it. For a pipe that is the same answer the pipeline form gave
// -- in `a | b | while ...` only the last pipe opens a compound.
func splitCompoundAfterOperator(line string) (string, string, string, bool) {
	if after, found := strings.CutPrefix(line, "!"); found {
		candidate := strings.TrimLeft(after, " \t")
		if len(candidate) < len(after) && beginsWithCompoundKeyword(candidate) {
			return "", "!", candidate, true
		}
	}
	for _, operator := range topLevelOperators(line) {
		candidate := strings.TrimLeft(line[operator.offset+len(operator.text):], " \t")
		if !beginsWithCompoundKeyword(candidate) {
			continue
		}
		before := strings.TrimSpace(line[:operator.offset])
		if before == "" {
			// `| while ...` or `&& if ...` with nothing in front is a syntax error, and
			// treating it as a stage or a term here would hide that.
			continue
		}
		return before, operator.text, candidate, true
	}
	return "", "", "", false
}

// joinCompoundPrefix puts a prefix back in front of text taken from the compound's header,
// for the passes that rewrite a header line and must leave the prefix where it was.
func joinCompoundPrefix(prefix, operator, text string) string {
	if operator == "!" {
		return "! " + text
	}
	return prefix + " " + operator + " " + text
}

// wrapCompoundAfterOperator joins the compound to the words before it, the way the operator
// between them says.
func wrapCompoundAfterOperator(node programNode, prefix, operator string, budget *parseBudget, depth int) (programNode, error) {
	switch operator {
	case "|":
		return wrapCompoundIntoPipeline(node, prefix, budget, depth)
	case "|&":
		// `cmd |& while ...` is `cmd 2>&1 | while ...`, and the prefix is parsed as a
		// line, so the redirection lands on its last command.
		return wrapCompoundIntoPipeline(node, prefix+" 2>&1", budget, depth)
	case "!":
		following := compoundAsList(node)
		following.items[0].value.pipelines[0].negated = true
		return listNode{value: following}, nil
	}
	prior, err := parseTypedLineWithBudget(prefix, budget, depth)
	if err != nil {
		return nil, err
	}
	if len(prior.items) == 0 {
		return nil, errMissingPipelineStage
	}
	following := compoundAsList(node)
	last := &prior.items[len(prior.items)-1]
	if operator == "&" {
		last.background = true
		prior.items = append(prior.items, following.items...)
		return listNode{value: prior}, nil
	}
	// `&&` or `||`: the compound's and-or continues the prefix's last one. Spliced rather
	// than nested, because a suffix may already have made the compound an and-or of its
	// own -- `true || while ...; done && echo x` -- and POSIX groups that to the left:
	// `(true || W) && echo x` runs the echo, where `true || { W && echo x; }` would not.
	kind := tokenAndIf
	if operator == "||" {
		kind = tokenOrIf
	}
	first := following.items[0]
	last.value.pipelines = append(last.value.pipelines, first.value.pipelines...)
	last.value.operators = append(append(last.value.operators, kind), first.value.operators...)
	last.background = first.background
	prior.items = append(prior.items, following.items[1:]...)
	return listNode{value: prior}, nil
}

// compoundAsList is the compound as a list: itself when a suffix has already made it one,
// and otherwise a one-stage pipeline holding it as a brace group -- which is how a compound
// used where a command is expected is represented everywhere else.
func compoundAsList(node programNode) list {
	if listed, ok := node.(listNode); ok && len(listed.value.items) > 0 && len(listed.value.items[0].value.pipelines) > 0 {
		return listed.value
	}
	group := braceGroup{body: Script{program: []programNode{node}}}
	return list{items: []listItem{{value: andOr{pipelines: []pipeline{{commands: []commandNode{group}}}}}}}
}

// parseFunctionAfterOperator reads `cmd && name() { ...; }` and `cmd & name() ...`. A function
// definition is a command too, so it defines the function exactly when that pipeline of the
// list would have run -- and after `&` it runs in the foreground, as the next item. Not after
// `|`: a definition as a pipeline stage would define it in that stage's subshell and nowhere.
func parseFunctionAfterOperator(line string, budget *parseBudget, depth int) (programNode, bool, error) {
	for _, operator := range topLevelOperators(line) {
		if operator.text == "|" || operator.text == "|&" {
			continue
		}
		prefix := strings.TrimSpace(line[:operator.offset])
		definitionLine, background := trailingBackground(strings.TrimSpace(line[operator.offset+len(operator.text):]))
		mayDefine := strings.Contains(definitionLine, "(") || strings.HasPrefix(definitionLine, "function")
		if prefix == "" || !mayDefine {
			continue
		}
		definition, recognized, err := parseDefinitionWithSuffix(definitionLine, budget, depth)
		if err != nil || !recognized {
			if err != nil {
				return nil, true, err
			}
			continue
		}
		node, err := wrapCompoundAfterOperator(definition, prefix, operator.text, budget, depth)
		if err != nil {
			return nil, true, err
		}
		if background {
			node = backgroundNode{value: node}
		}
		return node, true, nil
	}
	return nil, false, nil
}

// parseDefinitionWithSuffix reads a definition that more of an and-or list follows: `f() {
// ...; } && f`. The body ends where its closing brace does, so the first top-level `&&` or
// `||` begins what comes after -- the ones inside the body are inside its braces. The whole
// remainder used to be taken as the body, which then was not a compound command, so the line
// was a syntax error in both positions: at the start of a line and after an operator.
func parseDefinitionWithSuffix(line string, budget *parseBudget, depth int) (programNode, bool, error) {
	text, operator, rest := line, "", ""
	for _, found := range topLevelOperators(line) {
		if found.text == "&&" || found.text == "||" {
			text = strings.TrimSpace(line[:found.offset])
			operator, rest = found.text, strings.TrimSpace(line[found.offset+len(found.text):])
			break
		}
	}
	definition, recognized, err := parseFunctionDefinition(text, budget, depth)
	if err != nil || !recognized {
		return nil, recognized, err
	}
	if operator == "" {
		return definition, true, nil
	}
	node, err := wrapCompoundBeforeOperator(definition, operator, rest, budget, depth)
	return node, true, err
}
