package runtime

import (
	"errors"
	"fmt"
	"strings"
)

// One line of readline's init syntax, as a `bind` argument and a line of ~/.inputrc are read:
// `"KEYS": function`, `"KEYS": "macro"`, `Name: ...` for a key by name, or `set VARIABLE
// VALUE`. A macro's text is read as a key sequence is, escapes and all.

// bindLine applies one such line to keymap, prefix before each sequence.
func (r Runtime) bindLine(keymap, prefix, line string) error {
	line = strings.TrimSpace(line)
	if line == "" || line[0] == '#' {
		return nil
	}
	if rest, ok := strings.CutPrefix(line, "set "); ok {
		r.setReadlineVariable(strings.TrimSpace(rest))
		return nil
	}
	keys, value, err := splitBindLine(line)
	if err != nil {
		return err
	}
	keys = append([]byte(prefix), keys...)
	value = strings.TrimSpace(value)
	if value != "" && (value[0] == '"' || value[0] == '\'') {
		text, err := unquoteBindValue(value)
		if err != nil {
			return err
		}
		// A macro may be empty, `"KEYS": ""`, which types nothing: atuin binds its chain key so.
		macro, err := parseKeyseq(text)
		if err != nil && text != "" {
			return err
		}
		r.bindings.set(keymap, keys, KeyBinding{Kind: BindMacro, Text: string(macro)})
		return nil
	}
	name, _, _ := strings.Cut(value, " ")
	switch {
	case readlineCarriesOut(name):
		r.bindings.set(keymap, keys, KeyBinding{Kind: BindFunction, Text: name})
	case readlineHas(name):
		return fmt.Errorf("`%s': a readline function this shell's line editor does not carry out", name)
	}
	// A name readline has not got binds nothing, and says nothing, as in bash.
	return nil
}

// bindCommand is bind -x's argument, `"KEYS": command`: the keys run the command, which a pair
// of double quotes around it may hold.
func (r Runtime) bindCommand(keymap, prefix, text string) error {
	keys, command, err := splitBindLine(strings.TrimSpace(text))
	if err != nil {
		return err
	}
	command = strings.TrimSpace(command)
	if len(command) >= 2 && command[0] == '"' && command[len(command)-1] == '"' {
		command = command[1 : len(command)-1]
	}
	r.bindings.set(keymap, append([]byte(prefix), keys...), KeyBinding{Kind: BindCommand, Text: command})
	return nil
}

// splitBindLine parts a line into the keys before its colon and what follows the colon.
func splitBindLine(line string) ([]byte, string, error) {
	if line != "" && (line[0] == '"' || line[0] == '\'') {
		end := closingQuote(line)
		if end < 0 {
			return nil, "", errors.New("no closing quote in " + line)
		}
		keys, err := parseKeyseq(line[1:end])
		if err != nil {
			return nil, "", err
		}
		rest, found := strings.CutPrefix(strings.TrimLeft(line[end+1:], " \t"), ":")
		if !found {
			return nil, "", fmt.Errorf("%s: missing colon separator", line)
		}
		return keys, rest, nil
	}
	name, rest, found := strings.Cut(line, ":")
	if !found {
		return nil, "", fmt.Errorf("%s: missing colon separator", line)
	}
	keys, ok := keyNamed(strings.TrimSpace(name))
	if !ok {
		return nil, "", fmt.Errorf("`%s': no key by that name", strings.TrimSpace(name))
	}
	return keys, rest, nil
}

// closingQuote is the index of the quote that closes the one line begins with, past any escaped
// one, or -1.
func closingQuote(line string) int {
	for index := 1; index < len(line); index++ {
		switch line[index] {
		case '\\':
			index++
		case line[0]:
			return index
		}
	}
	return -1
}

// unquoteBindValue is what a quoted macro holds.
func unquoteBindValue(value string) (string, error) {
	end := closingQuote(value)
	if end < 0 {
		return "", errors.New("no closing quote in " + value)
	}
	return value[1:end], nil
}

// setReadlineVariable is `set NAME VALUE`. A name readline has not got is passed over, as
// readline passes over one; a switch takes on, off, 1 or 0. editing-mode is set -o vi or emacs.
func (r Runtime) setReadlineVariable(text string) {
	name, value, _ := strings.Cut(text, " ")
	value = strings.TrimSpace(value)
	current := readlineVariableDefault(name)
	if current == "" {
		return
	}
	if current == "on" || current == "off" {
		value, _, _ = strings.Cut(value, " ")
		if strings.EqualFold(value, "on") || value == "1" {
			value = "on"
		} else {
			value = "off"
		}
	}
	if name == "editing-mode" && (value == "vi" || value == "emacs") {
		r.options.vi, r.options.emacs = value == "vi", value == "emacs"
	}
	r.bindings.mu.Lock()
	r.bindings.variables[name] = value
	r.bindings.mu.Unlock()
}
