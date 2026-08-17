package completionspec

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// The reader in toml.go is a hand-written subset of TOML, and the question worth
// asking about it is not whether it parses -- the tests beside this one ask that
// -- but whether it reads a file the *same way a real TOML implementation does*.
//
// So a real one is kept here, in the test. BurntSushi/toml stays in go.mod for the
// behavior corpus, which is test-only code, and this costs the shipped binary
// nothing: the entire point of the hand-written reader is that the library's
// package init -- 22 ms of Windows registry enumeration to name a timezone, on
// every invocation -- is not linked into `nemosh`. A test binary can afford what a
// shell's startup cannot, and having the reference here is what keeps the subset
// from drifting into a dialect.
//
// The contract asserted is one-directional and deliberately so: *anything this
// reader accepts, the reference must accept and agree about*. The converse is
// false on purpose -- the reference accepts numbers, booleans and inline tables,
// and this format refuses them by name. Those refusals are asserted separately
// below, so that "we are stricter" stays a decision rather than becoming a gap.

// decodeWithReference is the same file through the library, into the same structs
// via the same `toml:` tags -- which is what those tags are still for.
func decodeWithReference(t *testing.T, source []byte) (Spec, error) {
	t.Helper()
	var spec Spec
	metadata, err := toml.Decode(string(source), &spec)
	if err != nil {
		return Spec{}, err
	}
	if undecoded := metadata.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, 0, len(undecoded))
		for _, key := range undecoded {
			keys = append(keys, key.String())
		}
		return Spec{}, &unknownKeysError{keys: keys}
	}
	return spec, nil
}

type unknownKeysError struct{ keys []string }

func (e *unknownKeysError) Error() string {
	return "keys this format does not have: " + strings.Join(e.keys, ", ")
}

// decodeWithReader is parseTOML plus decodeDocument: the shipped path, short of
// Validate, which is about the claims rather than the reading.
func decodeWithReader(source []byte) (Spec, error) {
	document, err := parseTOML(source)
	if err != nil {
		return Spec{}, err
	}
	return decodeDocument(document)
}

// Slices are compared with slices.Equal rather than reflect.DeepEqual, because a
// key that is absent and a key set to `[]` are both "no long options" and the two
// decoders are entitled to spell that as nil or as an empty slice.
func equalSurfaces(a, b Command) bool {
	return a.Name == b.Name && a.Operand == b.Operand &&
		a.Short == b.Short && a.ValueShort == b.ValueShort && a.FileShort == b.FileShort &&
		slices.Equal(a.Long, b.Long) && slices.Equal(a.ValueLong, b.ValueLong) &&
		slices.Equal(a.FileLong, b.FileLong)
}

func equalSpecs(a, b Spec) bool {
	if a.Meta != b.Meta || !equalSurfaces(a.Command, b.Command) {
		return false
	}
	if len(a.Subcommand) != len(b.Subcommand) {
		return false
	}
	for index := range a.Subcommand {
		if !equalSurfaces(Command(a.Subcommand[index]), Command(b.Subcommand[index])) {
			return false
		}
	}
	return true
}

// The bundled specs are the files that actually matter, and between them they
// carry every shape the format uses: single-line arrays of three names and of
// ninety, arrays broken over six lines with a trailing comma, comments on their
// own line and after a value, and a spec with no subcommands at all.
func TestReader_agreesWithTheReference_onEveryBundledSpec(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "..", "completions"))
	if err != nil {
		t.Fatalf("read the completions directory: %v", err)
	}
	found := 0
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".toml" {
			continue
		}
		found++
		t.Run(entry.Name(), func(t *testing.T) {
			// Given
			source, err := os.ReadFile(filepath.Join("..", "..", "completions", entry.Name()))
			if err != nil {
				t.Fatalf("read %s: %v", entry.Name(), err)
			}

			// When
			ours, ourErr := decodeWithReader(source)
			reference, referenceErr := decodeWithReference(t, source)

			// Then
			if ourErr != nil {
				t.Fatalf("this reader refused a bundled spec: %v", ourErr)
			}
			if referenceErr != nil {
				t.Fatalf("the reference refused a bundled spec: %v", referenceErr)
			}
			if !equalSpecs(ours, reference) {
				t.Fatalf("the two readers disagree about %s:\n ours: %+v\n ref:  %+v",
					entry.Name(), ours, reference)
			}
		})
	}
	if found == 0 {
		t.Fatal("no bundled specs were found, so this test proved nothing")
	}
	t.Logf("%d bundled specs read identically by both", found)
}

// The shapes a contributor could reasonably write that the bundled files happen
// not to contain. Each is valid TOML, so the reference is the authority on what
// it means and this reader has to match it.
func TestReader_agreesWithTheReference_onAwkwardButValidFiles(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "an empty file", source: ""},
		{name: "only comments", source: "# nothing here\n# nor here\n"},
		{name: "no trailing newline", source: "[command]\nname = \"a\""},
		{name: "CRLF line endings", source: "[meta]\r\nderived-from = \"x\"\r\n\r\n[command]\r\nname = \"a\"\r\n"},
		{name: "blank lines everywhere", source: "\n\n[command]\n\n\nname = \"a\"\n\n\n"},
		{name: "a comment after a value", source: "[command]\nname = \"a\" # the command\n"},
		{name: "a comment after a header", source: "[command] # the surface\nname = \"a\"\n"},
		{name: "an empty list", source: "[command]\nname = \"a\"\nlong = []\n"},
		{name: "an empty string", source: "[command]\nname = \"\"\n"},
		{name: "a one-element list", source: "[command]\nname = \"a\"\nlong = [\"verbose\"]\n"},
		{
			name:   "a list broken over lines with a trailing comma",
			source: "[command]\nname = \"a\"\nlong = [\n  \"one\",\n  \"two\",\n]\n",
		},
		{
			name:   "a list with a comment inside it",
			source: "[command]\nname = \"a\"\nlong = [\n  \"one\", # the first\n  \"two\",\n]\n",
			// A comment inside an array is valid TOML and the natural place to
			// explain one entry of ninety.
		},
		{name: "a list with no spaces", source: "[command]\nname = \"a\"\nlong = [\"one\",\"two\"]\n"},
		{name: "a literal string", source: "[command]\nname = 'a'\n"},
		{name: "an escaped quote", source: "[meta]\nderived-from = \"a \\\"b\\\" c\"\n"},
		{name: "an escaped backslash", source: "[meta]\nderived-from = \"a\\\\b\"\n"},
		{name: "a tab escape", source: "[meta]\nderived-from = \"a\\tb\"\n"},
		{name: "a unicode escape", source: "[meta]\nderived-from = \"a\\u00e9b\"\n"},
		{name: "a long unicode escape", source: "[meta]\nderived-from = \"a\\U0001F600b\"\n"},
		{name: "a non-ASCII value written directly", source: "[meta]\nderived-from = \"版本 1.0\"\n"},
		{name: "keys with underscores", source: "[command]\nname = \"a\"\n"},
		{name: "tabs as separators", source: "[command]\nname\t=\t\"a\"\n"},
		{name: "no subcommands", source: "[command]\nname = \"a\"\n"},
		{
			name:   "several subcommands",
			source: "[command]\nname = \"a\"\n[[subcommand]]\nname = \"one\"\n[[subcommand]]\nname = \"two\"\n",
		},
		{
			name:   "a subcommand with every field",
			source: "[command]\nname = \"a\"\n[[subcommand]]\nname = \"s\"\noperand = \"path\"\nshort = \"ab\"\nvalue-short = \"a\"\nfile-short = \"a\"\nlong = [\"x\"]\nvalue-long = [\"x\"]\nfile-long = [\"x\"]\n",
		},
		{
			name:   "meta and command in the other order",
			source: "[command]\nname = \"a\"\n\n[meta]\nderived-from = \"x\"\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			ours, ourErr := decodeWithReader([]byte(test.source))
			reference, referenceErr := decodeWithReference(t, []byte(test.source))

			// Then
			if ourErr != nil {
				t.Fatalf("this reader refused valid TOML: %v\nsource:\n%s", ourErr, test.source)
			}
			if referenceErr != nil {
				t.Fatalf("the case is not valid TOML, so it proves nothing: %v", referenceErr)
			}
			if !equalSpecs(ours, reference) {
				t.Fatalf("the two readers disagree:\n ours: %+v\n ref:  %+v\nsource:\n%s",
					ours, reference, test.source)
			}
		})
	}
}

// What this reader refuses that the reference would have accepted. Each is a part
// of TOML the format has no field for, and each is refused with a diagnostic that
// says which part -- because someone writing a spec has every reason to expect a
// number to work, and the useful answer is that it would have nowhere to go.
func TestReader_refusesTheTOMLThisFormatDoesNotHave(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		fragment string
	}{
		{name: "a number", source: "[command]\ntimeout = 30\n", fragment: "no numbers"},
		{name: "a boolean", source: "[command]\nquiet = true\n", fragment: "a boolean"},
		{name: "a datetime", source: "[meta]\nmeasured-on = 2026-08-16\n", fragment: "no numbers"},
		{name: "an inline table", source: "[command]\nname = { a = \"b\" }\n", fragment: "no inline tables"},
		{
			name:   "a multi-line string",
			source: "[meta]\nderived-from = \"\"\"\nlong\n\"\"\"\n",
			// Refused rather than read, because every value here is a name, a
			// letter set or a date, and none of those span lines.
			fragment: "fits on one line",
		},
		{name: "a dotted key", source: "[command]\na.b = \"c\"\n", fragment: "not followed by ="},
		{name: "a quoted key", source: "[command]\n\"name\" = \"a\"\n", fragment: "expected a key"},
		{name: "a nested table", source: "[command.sub]\nname = \"a\"\n", fragment: "not closed with ]"},
		{name: "a list of lists", source: "[command]\nlong = [[\"a\"]]\n", fragment: "holds strings"},
		{name: "a list of numbers", source: "[command]\nlong = [1, 2]\n", fragment: "holds strings"},
		{
			name:   "a mixed list",
			source: "[command]\nlong = [\"a\", 2]\n",
			// TOML 1.0 allows a heterogeneous array; this format's lists are all
			// names, so the second element has nowhere to go.
			fragment: "holds strings",
		},
		{name: "a key at the root", source: "name = \"a\"\n", fragment: "before any [table] header"},
		{name: "a value that is a list where a string goes", source: "[command]\nname = [\"a\"]\n", fragment: "wants a string"},
		{name: "a string where a list goes", source: "[command]\nlong = \"a\"\n", fragment: "wants a list of strings"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, err := decodeWithReader([]byte(test.source))

			// Then
			if err == nil {
				t.Fatalf("this reader accepted %q, which this format has no field for", test.source)
			}
			if !strings.Contains(err.Error(), test.fragment) {
				t.Fatalf("error = %q, want it to contain %q so the author is told which part is missing",
					err.Error(), test.fragment)
			}
		})
	}
}

// Malformed files, where the reader's job is to say where rather than to guess.
// None of these are valid TOML, so the reference is not the authority here; what
// is asserted is that the diagnostic carries a line number, because a spec file
// is ninety lines long and "syntax error" is not a place.
func TestReader_reportsWhereAFileIsMalformed(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		fragment string
	}{
		{name: "an unclosed string", source: "[command]\nname = \"a\n", fragment: "not closed before the end of the line"},
		{name: "a string never closed", source: "[command]\nname = \"a", fragment: "never closed"},
		{name: "an unclosed list", source: "[command]\nlong = [\"a\"\n", fragment: "never closed"},
		{name: "an unclosed header", source: "[command\nname = \"a\"\n", fragment: "not closed with ]"},
		{name: "a missing value", source: "[command]\nname =\n", fragment: "value is missing"},
		{name: "a missing equals", source: "[command]\nname \"a\"\n", fragment: "not followed by ="},
		{name: "two values on a line", source: "[command]\nname = \"a\" \"b\"\n", fragment: "one thing on a line"},
		{name: "a repeated key", source: "[command]\nname = \"a\"\nname = \"b\"\n", fragment: "set twice"},
		{name: "a repeated table", source: "[command]\n[command]\n", fragment: "appears twice"},
		{name: "a bad escape", source: "[command]\nname = \"a\\qb\"\n", fragment: "is not an escape"},
		{name: "a short unicode escape", source: "[command]\nname = \"a\\u12\"\n", fragment: "hex digits"},
		{name: "a surrogate half", source: "[command]\nname = \"a\\ud800\"\n", fragment: "not a character"},
		{name: "a missing comma", source: "[command]\nlong = [\"a\" \"b\"]\n", fragment: "expected , or ]"},
		{
			name:   "a table that is also a list of tables",
			source: "[subcommand]\nname = \"a\"\n[[subcommand]]\nname = \"b\"\n",
			// TOML forbids it, and silently merging them would give a spec one
			// subcommand where its author wrote two.
			fragment: "already a",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, err := decodeWithReader([]byte(test.source))

			// Then
			if err == nil {
				t.Fatalf("this reader accepted the malformed file %q", test.source)
			}
			if !strings.Contains(err.Error(), test.fragment) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), test.fragment)
			}
			if !strings.Contains(err.Error(), "line ") {
				t.Fatalf("error = %q, want a line number: a spec is ninety lines long and %q is not a place",
					err.Error(), err.Error())
			}
		})
	}
}

// The line number has to be the right one, or it is worse than none: it sends the
// reader to a line that is fine.
func TestReader_countsLinesThroughEverythingItSkips(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{name: "past blank lines", source: "[command]\n\n\n\nname = 1\n", want: "line 5"},
		{name: "past comments", source: "[command]\n# one\n# two\nname = 1\n", want: "line 4"},
		{
			name:   "past a list broken over lines",
			source: "[command]\nlong = [\n  \"a\",\n  \"b\",\n]\nname = 1\n",
			want:   "line 6",
		},
		{
			name:   "past a comment inside a list",
			source: "[command]\nlong = [\n  \"a\", # note\n  \"b\",\n]\nname = 1\n",
			want:   "line 6",
		},
		{name: "past CRLF endings", source: "[command]\r\n\r\nname = 1\r\n", want: "line 3"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, err := decodeWithReader([]byte(test.source))

			// Then
			if err == nil {
				t.Fatal("expected the bad value to be refused")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %q, want it to name %s", err.Error(), test.want)
			}
		})
	}
}

// Every unknown key at once, not the first: a file with three misspellings should
// take one pass to fix, not three.
func TestReader_namesEveryUnknownKeyTogether(t *testing.T) {
	// Given
	source := "[meta]\nauthr = \"x\"\n\n[command]\nname = \"a\"\nshortt = \"ab\"\n\n[[subcommand]]\nname = \"s\"\nlongg = [\"x\"]\n"

	// When
	_, err := decodeWithReader([]byte(source))

	// Then
	if err == nil {
		t.Fatal("expected the misspelled keys to be refused")
	}
	for _, want := range []string{"meta.authr", "command.shortt", "subcommand.longg"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to name %s", err.Error(), want)
		}
	}
}
