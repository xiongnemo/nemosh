package runtime

import "strings"

// scriptPrinter writes a parsed program back out as shell text, which is what `declare -f`
// and `type` show of a function. The result reads back as the same program; it is not the
// text as written. bash does the same -- it prints from what it parsed, four spaces a
// level -- and a script's `eval "$(declare -f f)"` or `nemosh -c "$(declare -f f); f"`
// wants something that runs, not a copy of the original's spacing.
//
// Statements go one to a line at block level; inside a group or a condition they go on
// one line with `;`. A heredoc's body is written after the line its operator is on, then
// its delimiter, which is where the shell reading it back will look.
type scriptPrinter struct {
	out     strings.Builder
	depth   int
	pending []redirectOperation
}

const printIndent = "    "

// printFunction is one definition in bash's shape: `name () `, then the body's group
// with its statements on lines of their own.
func printFunction(definition functionDefinition) string {
	var printer scriptPrinter
	printer.function(definition)
	return printer.out.String()
}

func (p *scriptPrinter) function(definition functionDefinition) {
	p.line(definition.name.value + " () ")
	if group, ok := definition.body.(braceGroup); ok {
		p.line("{ ")
		p.depth++
		p.statements(group.body.program)
		p.depth--
		p.line("}" + p.redirects(group.redirects))
		return
	}
	p.line(p.command(definition.body))
}

func (p *scriptPrinter) line(text string) {
	p.out.WriteString(strings.Repeat(printIndent, p.depth))
	p.out.WriteString(text)
	p.out.WriteByte('\n')
	for _, heredoc := range p.pending {
		p.out.WriteString(heredoc.body)
		if heredoc.body != "" && !strings.HasSuffix(heredoc.body, "\n") {
			p.out.WriteByte('\n')
		}
		p.out.WriteString(heredoc.delimiter + "\n")
	}
	p.pending = nil
}

func (p *scriptPrinter) statements(nodes []programNode) {
	for _, node := range nodes {
		p.statement(node)
	}
}

func (p *scriptPrinter) statement(node programNode) {
	switch value := node.(type) {
	case functionDefinition:
		p.function(value)
	case coprocNode:
		p.coproc(value)
	case ifNode:
		p.ifBlock(value, "if ")
	case loopNode:
		p.line(p.loopHeader(value))
		p.line("do")
		p.depth++
		p.statements(value.body)
		p.depth--
		p.line("done")
	case caseNode:
		p.line("case " + printWord(value.word) + " in ")
		p.depth++
		for _, arm := range value.arms {
			p.line(p.casePatterns(arm) + ")")
			p.depth++
			p.statements(arm.body)
			p.line(arm.terminator)
			p.depth--
		}
		p.depth--
		p.line("esac")
	default:
		p.line(p.inline(node))
	}
}

// ifBlock prints an if, and an else that holds nothing but another if as the elif it was
// written as: the parser turns elif into exactly that nesting (parser_elif.go).
func (p *scriptPrinter) ifBlock(node ifNode, keyword string) {
	p.line(keyword + p.list(node.condition) + "; then")
	p.depth++
	p.statements(node.thenBody)
	p.depth--
	if len(node.elseBody) == 1 {
		if nested, ok := node.elseBody[0].(ifNode); ok {
			p.ifBlock(nested, "elif ")
			return
		}
	}
	if len(node.elseBody) > 0 {
		p.line("else")
		p.depth++
		p.statements(node.elseBody)
		p.depth--
	}
	p.line("fi")
}

func (p *scriptPrinter) loopHeader(node loopNode) string {
	switch node.kind {
	case loopArithmetic:
		return "for ((" + node.arith.initialize + "; " + node.arith.condition + "; " + node.arith.step + "))"
	case loopWhile, loopUntil:
		keyword := "while "
		if node.kind == loopUntil {
			keyword = "until "
		}
		return keyword + p.list(node.condition)
	}
	keyword := "for "
	if node.kind == loopSelect {
		keyword = "select "
	}
	if node.overArguments {
		return keyword + node.name
	}
	words := make([]string, len(node.values))
	for index, value := range node.values {
		words[index] = printWord(value)
	}
	return keyword + node.name + " in " + strings.Join(words, " ")
}

func (p *scriptPrinter) casePatterns(arm caseArmNode) string {
	patterns := make([]string, len(arm.patterns))
	for index, pattern := range arm.patterns {
		patterns[index] = printWord(pattern)
	}
	return strings.Join(patterns, " | ")
}

// inline is a statement on one line: what a group or a condition holds.
func (p *scriptPrinter) inline(node programNode) string {
	switch value := node.(type) {
	case listNode:
		return p.list(value.value)
	case backgroundNode:
		return p.inline(value.value) + " &"
	case functionDefinition:
		return value.name.value + " () " + p.command(value.body)
	case ifNode:
		text := "if " + p.list(value.condition) + "; then " + p.inlineStatements(value.thenBody)
		if len(value.elseBody) > 0 {
			text += " else " + p.inlineStatements(value.elseBody)
		}
		return text + " fi"
	case loopNode:
		return p.loopHeader(value) + "; do " + p.inlineStatements(value.body) + " done"
	case caseNode:
		text := "case " + printWord(value.word) + " in"
		for _, arm := range value.arms {
			text += " " + p.casePatterns(arm) + ") " + strings.TrimSuffix(p.inlineStatements(arm.body), ";") + " " + arm.terminator
		}
		return text + " esac"
	}
	return ""
}

// inlineStatements is statements for a one-line body, each ended with `;` so a closer can
// follow: `{ a; b; }`.
func (p *scriptPrinter) inlineStatements(nodes []programNode) string {
	var parts []string
	for _, node := range nodes {
		text := p.inline(node)
		if !strings.HasSuffix(text, "&") {
			text += ";"
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, " ")
}

func (p *scriptPrinter) list(value list) string {
	var out strings.Builder
	for index, item := range value.items {
		if index > 0 {
			if value.items[index-1].background {
				out.WriteByte(' ')
			} else {
				out.WriteString("; ")
			}
		}
		out.WriteString(p.andOr(item.value))
		if item.background {
			out.WriteString(" &")
		}
	}
	return out.String()
}

func (p *scriptPrinter) andOr(value andOr) string {
	var out strings.Builder
	for index, pipeline := range value.pipelines {
		if index > 0 {
			if value.operators[index-1] == tokenOrIf {
				out.WriteString(" || ")
			} else {
				out.WriteString(" && ")
			}
		}
		if pipeline.negated {
			out.WriteString("! ")
		}
		commands := make([]string, len(pipeline.commands))
		for position, command := range pipeline.commands {
			commands[position] = p.command(command)
		}
		out.WriteString(strings.Join(commands, " | "))
	}
	return out.String()
}

func (p *scriptPrinter) command(node commandNode) string {
	switch value := node.(type) {
	case simpleCommand:
		words := make([]string, len(value.words))
		for index, word := range value.words {
			words[index] = printWord(word)
		}
		return strings.Join(words, " ") + p.redirects(value.redirects)
	case braceGroup:
		return "{ " + p.inlineStatements(value.body.program) + " }" + p.redirects(value.redirects)
	case subshellCommand:
		return "( " + strings.TrimSuffix(p.inlineStatements(value.body.program), ";") + " )" + p.redirects(value.redirects)
	}
	return ""
}
