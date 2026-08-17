package completionspec

import (
	"fmt"
	"slices"
	"strings"
)

// validateSurface holds the rules that apply equally to a command and to a
// subcommand, which is all of them: a subcommand is a surface like any other.
func validateSurface(where string, operand OperandKind, short, valueShort, fileShort string, long, valueLong, fileLong []string) error {
	switch operand {
	case "", OperandPath, OperandDirectory, OperandHost, OperandNone:
	default:
		return fmt.Errorf("%s: operand %q is not one of path, directory, host, none", where, operand)
	}
	if letter, duplicated := firstRepeat(short); duplicated {
		return fmt.Errorf("%s: option -%c is listed twice", where, letter)
	}
	// The two narrow columns are subsets of the wide one, stated as such, so a
	// letter cannot be claimed as taking a file while not being an option at all.
	for _, letter := range valueShort {
		if !strings.ContainsRune(short, letter) {
			return fmt.Errorf("%s: -%c takes a value but is not in short", where, letter)
		}
	}
	for _, letter := range fileShort {
		if !strings.ContainsRune(valueShort, letter) {
			return fmt.Errorf("%s: -%c takes a file but not a value", where, letter)
		}
	}
	for _, name := range long {
		if strings.HasPrefix(name, "-") {
			return fmt.Errorf("%s: long option %q is written with its dashes; write the name alone", where, name)
		}
		if name == "" {
			return fmt.Errorf("%s: an empty long option name", where)
		}
	}
	if name, duplicated := firstRepeatedName(long); duplicated {
		return fmt.Errorf("%s: long option --%s is listed twice", where, name)
	}
	for _, name := range valueLong {
		if !slices.Contains(long, name) {
			return fmt.Errorf("%s: --%s takes a value but is not in long", where, name)
		}
	}
	for _, name := range fileLong {
		if !slices.Contains(valueLong, name) {
			return fmt.Errorf("%s: --%s takes a file but not a value", where, name)
		}
	}
	return nil
}

func firstRepeat(letters string) (rune, bool) {
	seen := map[rune]bool{}
	for _, letter := range letters {
		if seen[letter] {
			return letter, true
		}
		seen[letter] = true
	}
	return 0, false
}

func firstRepeatedName(names []string) (string, bool) {
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			return name, true
		}
		seen[name] = true
	}
	return "", false
}

// Parse reads one spec and checks it, refusing anything it does not fully
// understand.
//
// Unknown keys are an error rather than a shrug. A misspelled key that is
// silently dropped leaves a file that looks correct, completes nothing, and
// gives its author no reason why -- the same argument the behavior corpus makes
// for rejecting unknown keys in its own cases.
func Parse(name string, source []byte) (Spec, error) {
	document, err := parseTOML(source)
	if err != nil {
		return Spec{}, fmt.Errorf("%s: %w", name, err)
	}
	spec, err := decodeDocument(document)
	if err != nil {
		return Spec{}, fmt.Errorf("%s: %w", name, err)
	}
	if err := spec.Validate(name); err != nil {
		return Spec{}, err
	}
	return spec, nil
}
