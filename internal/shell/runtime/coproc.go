package runtime

import "strings"

// coprocLine reads a line that begins with bash's `coproc` as that word alone. The form is
// refused when it runs (unimplemented.go), and what follows it -- `coproc NAME { cat; }` --
// is a compound in a position no other command puts one, which the parser reported as
// `unexpected }` and so stopped the whole script before its first line. Read this way, the
// refusal names the form, and only when it is reached.
func coprocLine(line string, budget *parseBudget) (programNode, bool) {
	if line != "coproc" && !strings.HasPrefix(line, "coproc ") && !strings.HasPrefix(line, "coproc\t") {
		return nil, false
	}
	command := simpleCommand{words: []word{{parts: []wordPart{{kind: wordPartLiteral, text: "coproc"}}}}, line: budget.line()}
	item := listItem{value: andOr{pipelines: []pipeline{{commands: []commandNode{command}}}}}
	return listNode{value: list{items: []listItem{item}}}, true
}
