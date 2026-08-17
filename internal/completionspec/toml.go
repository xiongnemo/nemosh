package completionspec

import (
	"fmt"
	"strings"
)

// The spec format is TOML, and this reads the part of TOML the format uses:
// tables, arrays of tables, string values, arrays of strings, comments. Nothing
// else, by design.
//
// Written by hand rather than taken from a library, for a measured reason.
// BurntSushi/toml -- which the behavior corpus still uses, and which the
// differential test beside this file checks this reader against -- initialises
// three package-level variables by calling `time.Now().Zone()`. On Windows the
// first such call makes Go's runtime translate the local zone's name by
// enumerating every subkey under
// `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Time Zones`, and package init
// runs whether or not anything ever decodes a file. Measured on the machine this
// was written on: 22 ms and 1,343 allocations before `main`, on every invocation
// -- `nemosh true`, `nemosh --version`, every applet shim -- against a 4.8 ms
// floor for a Go binary that only prints. Three quarters of this shell's startup,
// spent deriving a timezone that no spec file has a datetime to need.
//
// So this is not a TOML implementation and does not want to be. It reads what the
// format uses and refuses the rest by name, which is the bargain the format
// already makes about unknown keys: a file half understood is worse than one
// refused, because it looks correct and completes nothing.

// tomlValue is one value: a string, or a list of them.
type tomlValue struct {
	text   string
	list   []string
	isList bool
}

// tomlTable is one `[table]`. The order keys were written in is kept, so a
// diagnostic names them in the order their author reads them.
type tomlTable struct {
	values map[string]tomlValue
	order  []string
}

func newTomlTable() *tomlTable {
	return &tomlTable{values: map[string]tomlValue{}}
}

func (t *tomlTable) set(key string, value tomlValue) {
	t.values[key] = value
	t.order = append(t.order, key)
}

func (t *tomlTable) keys() []string { return t.order }

// tomlDocument is a whole file: the plain tables, and the arrays of tables that
// `[[subcommand]]` builds.
type tomlDocument struct {
	tables map[string]*tomlTable
	arrays map[string][]*tomlTable
	order  []string
}

type tomlReader struct {
	source []byte
	pos    int
	line   int
}

// parseTOML reads a whole file.
func parseTOML(source []byte) (*tomlDocument, error) {
	reader := &tomlReader{source: source, line: 1}
	document := &tomlDocument{tables: map[string]*tomlTable{}, arrays: map[string][]*tomlTable{}}
	var current *tomlTable
	currentName := ""
	for {
		reader.skipSpaceAndComments()
		if reader.done() {
			return document, nil
		}
		if reader.source[reader.pos] == '[' {
			table, name, err := reader.readTableHeader(document)
			if err != nil {
				return nil, err
			}
			current, currentName = table, name
			continue
		}
		if current == nil {
			// A key with no table above it would belong to TOML's root table,
			// which this format does not use: every value lives under [meta],
			// [command] or [[subcommand]].
			return nil, fmt.Errorf("line %d: %q comes before any [table] header", reader.line, reader.rest())
		}
		key, value, err := reader.readAssignment()
		if err != nil {
			return nil, err
		}
		if _, repeated := current.values[key]; repeated {
			return nil, fmt.Errorf("line %d: %s is set twice in [%s]", reader.line, key, currentName)
		}
		current.set(key, value)
	}
}

// readTableHeader reads `[name]` or `[[name]]` and registers the table it opens.
func (r *tomlReader) readTableHeader(document *tomlDocument) (*tomlTable, string, error) {
	start := r.line
	r.pos++
	isArray := r.pos < len(r.source) && r.source[r.pos] == '['
	if isArray {
		r.pos++
	}
	r.skipBlanks()
	name, err := r.readKey()
	if err != nil {
		return nil, "", err
	}
	r.skipBlanks()
	closing := "]"
	if isArray {
		closing = "]]"
	}
	if !r.consume(closing) {
		return nil, "", fmt.Errorf("line %d: the header for %s is not closed with %s", start, name, closing)
	}
	if err := r.endOfLine(); err != nil {
		return nil, "", err
	}
	table := newTomlTable()
	if isArray {
		if _, taken := document.tables[name]; taken {
			return nil, "", fmt.Errorf("line %d: %s is already a [%s] table, so [[%s]] cannot also be a list of them", start, name, name, name)
		}
		if len(document.arrays[name]) == 0 {
			document.order = append(document.order, name)
		}
		document.arrays[name] = append(document.arrays[name], table)
		return table, name, nil
	}
	if _, repeated := document.tables[name]; repeated {
		return nil, "", fmt.Errorf("line %d: [%s] appears twice", start, name)
	}
	if len(document.arrays[name]) > 0 {
		return nil, "", fmt.Errorf("line %d: %s is already a list of [[%s]] tables", start, name, name)
	}
	document.tables[name] = table
	document.order = append(document.order, name)
	return table, name, nil
}

// readAssignment reads one `key = value` line.
func (r *tomlReader) readAssignment() (string, tomlValue, error) {
	key, err := r.readKey()
	if err != nil {
		return "", tomlValue{}, err
	}
	r.skipBlanks()
	if !r.consume("=") {
		// The likely cause is a key spelled with something this format's keys do
		// not contain -- a dot, a quote -- so the reader stopped early and what
		// follows is not the `=` it wanted.
		return "", tomlValue{}, fmt.Errorf("line %d: %s is not followed by = but by %q", r.line, key, r.rest())
	}
	r.skipBlanks()
	value, err := r.readValue()
	if err != nil {
		return "", tomlValue{}, err
	}
	if err := r.endOfLine(); err != nil {
		return "", tomlValue{}, err
	}
	return key, value, nil
}

// readKey reads a bare key. TOML's quoted and dotted keys are left out: this
// format's keys are all `lower-case-with-dashes`, and a file needing either is
// saying something the format has no field for.
func (r *tomlReader) readKey() (string, error) {
	start := r.pos
	for r.pos < len(r.source) && isBareKeyByte(r.source[r.pos]) {
		r.pos++
	}
	if r.pos == start {
		return "", fmt.Errorf("line %d: expected a key, found %q", r.line, r.rest())
	}
	return string(r.source[start:r.pos]), nil
}

func isBareKeyByte(char byte) bool {
	switch {
	case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
		return true
	case char == '-' || char == '_':
		return true
	}
	return false
}

// skipSpaceAndComments passes over blanks, line breaks and comments. Used
// between items and inside a list, which is what lets a long list of long
// options be written over several lines.
func (r *tomlReader) skipSpaceAndComments() {
	for r.pos < len(r.source) {
		switch r.source[r.pos] {
		case '\n':
			r.line++
			r.pos++
		case ' ', '\t', '\r':
			r.pos++
		case '#':
			r.skipToLineEnd()
		default:
			return
		}
	}
}

func (r *tomlReader) skipBlanks() {
	for r.pos < len(r.source) && (r.source[r.pos] == ' ' || r.source[r.pos] == '\t') {
		r.pos++
	}
}

func (r *tomlReader) skipToLineEnd() {
	for r.pos < len(r.source) && r.source[r.pos] != '\n' {
		r.pos++
	}
}

// endOfLine insists nothing follows but blanks and a comment.
//
// TOML puts one assignment on a line. Accepting a second silently would let a
// spec carry a claim its author could not see, which is the failure this whole
// format is arranged against.
func (r *tomlReader) endOfLine() error {
	for r.pos < len(r.source) {
		switch r.source[r.pos] {
		case ' ', '\t', '\r':
			r.pos++
		case '\n':
			// Left for skipSpaceAndComments, which is what counts lines.
			return nil
		case '#':
			r.skipToLineEnd()
			return nil
		default:
			return fmt.Errorf("line %d: %q follows a complete value; this format puts one thing on a line", r.line, r.rest())
		}
	}
	return nil
}

func (r *tomlReader) done() bool { return r.pos >= len(r.source) }

func (r *tomlReader) consume(text string) bool {
	if strings.HasPrefix(string(r.source[r.pos:]), text) {
		r.pos += len(text)
		return true
	}
	return false
}

// rest is the remainder of the line, for a diagnostic, cut so a pasted-in
// thousand-character array does not become the error message.
func (r *tomlReader) rest() string {
	end := r.pos
	for end < len(r.source) && r.source[end] != '\n' && end-r.pos < 32 {
		end++
	}
	if end == r.pos {
		return "the end of the file"
	}
	return string(r.source[r.pos:end])
}
