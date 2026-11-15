package main

import (
	"fmt"
	"os"
	"strings"
)

// completeVariable completes `$NA` or `${NA` at the cursor to a variable's name, and reports
// whether the word was one to complete that way.
//
// It works on the text as typed, unlike the path completion beside it: the dollar sign has to
// stay a dollar sign, where inserting a name as a path would escape it. A unique name is
// followed by what bash puts after it: `/` when the variable holds a directory, so `$HOME/`
// goes straight on into it, and otherwise a blank -- unless a double quote is still open, when
// a blank would land inside the string.
func (e *lineEditor) completeVariable(prompt string) bool {
	if e.variables == nil {
		return false
	}
	word := e.buffer.currentWord()
	dollar := strings.LastIndexByte(word, '$')
	if dollar < 0 || dollar > 0 && word[dollar-1] == '\\' {
		return false
	}
	_, open := typedWordValue(word[:dollar])
	if open == '\'' {
		// In single quotes a dollar sign is only a character.
		return false
	}
	typed := word[dollar+1:]
	braced := strings.HasPrefix(typed, "{")
	stem := strings.TrimPrefix(typed, "{")
	if !isNamePrefix(stem) {
		return false
	}
	var names []string
	for _, name := range e.variables() {
		if strings.HasPrefix(name, stem) {
			names = append(names, name)
		}
	}
	spell := func(name string) string {
		if braced {
			return "${" + name
		}
		return "$" + name
	}
	switch len(names) {
	case 0:
		fmt.Fprint(e.screen, "\a")
	case 1:
		completed := spell(names[0])
		if braced {
			completed += "}"
		}
		e.replaceWord(word[dollar:], completed+e.afterVariable(names[0], open))
	default:
		if shared := longestSharedPrefix(names); shared != stem {
			e.replaceWord(word[dollar:], spell(shared))
		}
		listed := make([]string, len(names))
		for index, name := range names {
			listed[index] = "$" + name
		}
		e.listCandidates(listed, prompt)
	}
	return true
}

// afterVariable is what follows a completed name: a separator into the directory it holds, a
// blank to start the next word, or nothing inside a double quote still open.
func (e *lineEditor) afterVariable(name string, open rune) string {
	if value, ok := e.variableValue(name); ok && value != "" {
		native, resolved := value, true
		if e.resolve != nil {
			native, resolved = e.resolve(value)
		}
		if info, err := os.Stat(native); resolved && err == nil && info.IsDir() {
			if strings.HasSuffix(value, "/") {
				return ""
			}
			return "/"
		}
	}
	if open == '"' {
		return ""
	}
	return " "
}

// isNamePrefix reports whether text could begin a variable name: empty, or a letter or
// underscore followed by letters, digits and underscores.
func isNamePrefix(text string) bool {
	for index, r := range text {
		letter := r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		if !letter && (index == 0 || r < '0' || r > '9') {
			return false
		}
	}
	return true
}
