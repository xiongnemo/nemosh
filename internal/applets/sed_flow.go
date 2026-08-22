package applets

import "fmt"

// Branching, and the flattening it requires.
//
// `b`, `t` and `T` jump to a label, and a label can sit inside a block that the
// jump comes from outside. A tree of commands cannot express that: walking it
// recursively means a jump has nowhere to land. So the tree is flattened into one
// instruction list at parse time, with a block becoming a conditional jump over
// its body -- which is what sed itself does, and the reason `:a;N;$!ba` works at
// all.
//
// This is also what makes `D` possible. It restarts the script *without* reading
// a new line, and a restart is a jump to instruction zero.

// flattenSedProgram turns the parsed tree into a linear program and resolves the
// labels.
//
// A `{` instruction carries the index just past its body, so a block whose
// address does not select is skipped in one step rather than walked and ignored.
func flattenSedProgram(program *sedProgram) error {
	program.instructions = appendSedInstructions(nil, program.commands)
	program.labels = map[string]int{}
	for index, command := range program.instructions {
		if command.action != ':' {
			continue
		}
		if command.label == "" {
			return fmt.Errorf("\":\" lacks a label")
		}
		if _, duplicate := program.labels[command.label]; duplicate {
			return fmt.Errorf("duplicate label '%s'", command.label)
		}
		program.labels[command.label] = index
	}
	for _, command := range program.instructions {
		switch command.action {
		case 'b', 't', 'T':
			if command.label == "" {
				// A bare branch goes to the end of the script, which is what
				// makes `s/a/X/;t;s/b/Y/` skip the second substitution.
				command.jump = len(program.instructions)
				continue
			}
			target, ok := program.labels[command.label]
			if !ok {
				return fmt.Errorf("can't find label for jump to `%s'", command.label)
			}
			command.jump = target
		}
	}
	return nil
}

func appendSedInstructions(into []*sedCommand, commands []*sedCommand) []*sedCommand {
	for _, command := range commands {
		into = append(into, command)
		if command.action != '{' {
			continue
		}
		into = appendSedInstructions(into, command.block)
		// Resolved after the body, so the jump lands on whatever follows it.
		command.jump = len(into)
	}
	return into
}

// runSedProgram walks the flattened program for one line.
//
// The program counter is what branching needs, and it is why this replaced the
// recursive walk: `b`, `t`, `T` and `D` all move it somewhere the walk had no way
// to express.
func runSedProgram(program *sedProgram, cycle *sedCycle) (sedControl, error) {
	counter := 0
	for counter < len(program.instructions) {
		command := program.instructions[counter]
		if command.action == ':' {
			counter++
			continue
		}
		if !command.address.selects(cycle.line, cycle.number, cycle.isLast) {
			if command.action == '{' {
				counter = command.jump
				continue
			}
			counter++
			continue
		}
		switch command.action {
		case '{':
			// Selected, so fall into the body.
			counter++
			continue
		case 'b':
			counter = command.jump
			continue
		case 't', 'T':
			// The flag records whether a substitution has happened since the
			// last input line was read or the last t/T ran, so testing it clears
			// it.
			substituted := cycle.substituted
			cycle.substituted = false
			if substituted == (command.action == 't') {
				counter = command.jump
				continue
			}
			counter++
			continue
		}
		control, err := runSedCommand(command, cycle)
		if err != nil {
			return control, err
		}
		switch control {
		case sedNext:
			counter++
		case sedRestart:
			// `D` with a newline left in the pattern space: start again without
			// reading, which is the loop that makes `N;P;D` a sliding window.
			counter = 0
			cycle.substituted = false
		default:
			return control, nil
		}
	}
	return sedNext, nil
}
