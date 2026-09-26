package oilsspec

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Spec is one spec file as test/sh_spec.py reads it: the metadata at its head, such as
// compare_shells, and its cases in order.
type Spec struct {
	Metadata map[string]string
	Cases    []Case
}

// Case is one case of a spec file: the #### line that begins it, its code, and what it
// expects. Default holds the expectations every shell is held to, keyed as Oils writes
// them (stdout, stdout-json, stderr, stderr-json, status). Shells holds the ones a
// qualified line such as `## N-I dash/ash status: 2` sets for particular shells.
type Case struct {
	Desc    string
	Line    int
	Code    string
	Default map[string]string
	Shells  map[string]*Qualified
}

// Qualified is what one shell is expected to do instead of the default, and the word
// that says how Oils regards it: OK, OK-2 to OK-4, BUG, BUG-2, or N-I, not implemented.
type Qualified struct {
	Qualifier string
	Values    map[string]string
}

// metaFields are the names sh_spec.py accepts at the head of a file.
var metaFields = []string{
	"our_shell", "compare_shells", "suite", "tags",
	"oils_failures_allowed", "oils_cpp_failures_allowed", "legacy_tmp_dir",
}

var (
	keyValueLine = regexp.MustCompile(`^##\s+(?:(OK(?:-\d)?|BUG(?:-\d)?|N-I)\s+([\w+/]+)\s+)?([\w\-]+):\s*(.*)`)
	endLine      = regexp.MustCompile(`^##\s+END`)
)

// Parse reads a spec file the way test/sh_spec.py does, and fails where it fails. The
// grammar is the one its comments give:
//
//	test_file = key_value* (test_case '\n')*
//	test_case = '####' DESC key_value* code key_value*
//	code      = PLAIN_LINE* | '## code:' VALUE
//	key_value = '##' KEY ':' VALUE | KEY_VALUE_MULTILINE PLAIN_LINE* END_MULTILINE
//
// As there, a line whose first non-blank character is # is a comment wherever it falls,
// inside code and expected output too, and the blank lines between cases are skipped
// while those inside code and output are kept.
func Parse(content string) (Spec, error) {
	tokens := &tokenizer{lines: strings.SplitAfter(content, "\n")}
	if err := tokens.next(lexOuter); err != nil {
		return Spec{}, err
	}
	spec := Spec{Metadata: map[string]string{}}
	for tokens.current.kind == tokenKeyValue {
		head := tokens.current
		if head.qualifier != "" {
			return Spec{}, fmt.Errorf("line %d: file metadata takes no qualifier", head.line)
		}
		spec.Metadata[head.name] = head.value
		if err := tokens.next(lexOuter); err != nil {
			return Spec{}, err
		}
	}
	for tokens.current.kind != tokenEOF {
		c, err := parseCase(tokens)
		if err != nil {
			return Spec{}, err
		}
		spec.Cases = append(spec.Cases, c)
	}
	for name := range spec.Metadata {
		if !slices.Contains(metaFields, name) {
			return Spec{}, fmt.Errorf("invalid file metadata %q", name)
		}
	}
	return spec, nil
}

func parseCase(tokens *tokenizer) (Case, error) {
	begin := tokens.current
	if begin.kind != tokenCaseBegin {
		return Case{}, fmt.Errorf("line %d: expected #### to begin a case", begin.line)
	}
	c := Case{Desc: begin.text, Line: begin.line, Default: map[string]string{}, Shells: map[string]*Qualified{}}
	if err := tokens.next(lexOuter); err != nil {
		return Case{}, err
	}
	if err := c.keyValues(tokens); err != nil {
		return Case{}, err
	}
	if _, given := c.Default["code"]; !given {
		if tokens.current.kind != tokenPlain {
			return Case{}, fmt.Errorf("line %d: expected a line of code", tokens.current.line)
		}
		var code strings.Builder
		for tokens.current.kind == tokenPlain {
			code.WriteString(tokens.current.text)
			if err := tokens.next(lexRaw); err != nil {
				return Case{}, err
			}
		}
		c.Code = code.String()
		if err := c.keyValues(tokens); err != nil {
			return Case{}, err
		}
	}
	// A `## code:` line wins, even one after the code, since sh_spec.py keeps both in
	// one dictionary.
	if code, given := c.Default["code"]; given {
		c.Code = code
		delete(c.Default, "code")
	}
	return c, nil
}

// keyValues reads the contiguous ## lines at the tokenizer, one-line and multi-line.
func (c *Case) keyValues(tokens *tokenizer) error {
	for {
		item := tokens.current
		switch item.kind {
		case tokenKeyValue:
			if err := c.set(item, item.name, item.value); err != nil {
				return err
			}
			if err := tokens.next(lexOuter); err != nil {
				return err
			}
		case tokenMultiline:
			if item.value != "" {
				return fmt.Errorf("line %d: got value %q for %q, but the value should be on the following lines", item.line, item.value, item.name)
			}
			var value strings.Builder
			for {
				if err := tokens.next(lexRaw); err != nil {
					return err
				}
				if tokens.current.kind != tokenPlain {
					break
				}
				value.WriteString(tokens.current.text)
			}
			if err := c.set(item, strings.ToLower(item.name), value.String()); err != nil {
				return err
			}
			// ## END is optional: the next ## line or case ends the value as well.
			if tokens.current.kind == tokenEnd {
				if err := tokens.next(lexOuter); err != nil {
					return err
				}
			}
		default:
			return nil
		}
	}
}

// set records one expectation, for every shell when unqualified, and otherwise for each
// shell the line names, refusing what sh_spec.py refuses: a second value for the same
// key, counting stdout and stdout-json as one, and a second qualifier for one shell.
func (c *Case) set(item token, name, value string) error {
	if item.qualifier == "" {
		c.Default[name] = value
		return nil
	}
	for shell := range strings.SplitSeq(item.shells, "/") {
		shellCase := c.Shells[shell]
		if shellCase == nil {
			shellCase = &Qualified{Qualifier: item.qualifier, Values: map[string]string{}}
			c.Shells[shell] = shellCase
		}
		key := strings.TrimSuffix(name, "-json")
		_, plain := shellCase.Values[key]
		_, encoded := shellCase.Values[key+"-json"]
		if plain || encoded {
			return fmt.Errorf("line %d: duplicate spec %q for %q", item.line, name, shell)
		}
		if shellCase.Qualifier != item.qualifier {
			return fmt.Errorf("line %d: inconsistent qualifier %q for %q, which was given %q", item.line, item.qualifier, shell, shellCase.Qualifier)
		}
		shellCase.Values[name] = value
	}
	return nil
}
