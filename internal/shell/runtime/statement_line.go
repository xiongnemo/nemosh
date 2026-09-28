package runtime

// statementLine is the source line a top-level statement begins on, and 0 for one that carries
// none: the line of its first command, or of a loop's or a case's header. It is how
// executeStatements finds the statements that shared a line with one it abandoned.
func statementLine(node programNode) int {
	switch value := node.(type) {
	case listNode:
		return listLine(value.value)
	case backgroundNode:
		return statementLine(value.value)
	case ifNode:
		return listLine(value.condition)
	case loopNode:
		return value.line
	case caseNode:
		return value.line
	}
	return 0
}

func listLine(items list) int {
	if len(items.items) == 0 || len(items.items[0].value.pipelines) == 0 {
		return 0
	}
	commands := items.items[0].value.pipelines[0].commands
	if len(commands) == 0 {
		return 0
	}
	switch command := commands[0].(type) {
	case simpleCommand:
		return command.line
	case braceGroup:
		if len(command.body.program) > 0 {
			return statementLine(command.body.program[0])
		}
	case subshellCommand:
		if len(command.body.program) > 0 {
			return statementLine(command.body.program[0])
		}
	}
	return 0
}
