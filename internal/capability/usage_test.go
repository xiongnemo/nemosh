package capability

import (
	"strings"
	"testing"
)

// The usage text is held to the matrix in both directions, which is what stops it
// from becoming a list of claims nobody checks.
//
// The matrix rows are themselves bound to behaviour by capability_test.go, so a
// letter reaching the help has been measured against a built binary. What is left
// to check here is that the two halves agree: an option with no gloss would print a
// blank line, and a gloss for an option that does not exist is a promise the applet
// will not keep.

func TestUsage_coversEveryApplet(t *testing.T) {
	for _, command := range commands {
		if command.Builtin {
			// Builtins are described by `help` and `type`, and the matrix claims
			// no options for them, so there is nothing here to render.
			continue
		}
		t.Run(command.Name, func(t *testing.T) {
			usage, ok := usageText[command.Name]
			if !ok {
				t.Fatalf("%s has no usage entry, so `%s --help` would say nothing",
					command.Name, command.Name)
			}
			if usage.Summary == "" {
				t.Fatalf("%s has no summary", command.Name)
			}
			// A summary is a sentence about what the thing does, so it reads like
			// one. This is the sort of rule that keeps a table of 58 entries
			// consistent when it is filled in over time.
			if !strings.HasSuffix(usage.Summary, ".") {
				t.Fatalf("%s: summary %q should end in a full stop", command.Name, usage.Summary)
			}
			if first := usage.Summary[0]; first < 'A' || first > 'Z' {
				t.Fatalf("%s: summary %q should start with a capital", command.Name, usage.Summary)
			}
		})
	}
}

func TestUsage_glossesEveryOptionAndNoOthers(t *testing.T) {
	for _, command := range commands {
		if command.Builtin {
			continue
		}
		usage := usageText[command.Name]
		t.Run(command.Name, func(t *testing.T) {
			for _, letter := range command.Short {
				gloss, ok := usage.Options[string(letter)]
				if !ok || gloss == "" {
					t.Fatalf("%s takes -%c and does not say what it does", command.Name, letter)
				}
			}
			for _, name := range command.Long {
				gloss, ok := usage.Options[name]
				if !ok || gloss == "" {
					t.Fatalf("%s takes --%s and does not say what it does", command.Name, name)
				}
			}
			// The other direction: a gloss with no option behind it.
			for spelling := range usage.Options {
				if len(spelling) == 1 && strings.Contains(command.Short, spelling) {
					continue
				}
				if containsName(command.Long, spelling) {
					continue
				}
				t.Fatalf("%s glosses %q, which it does not accept", command.Name, spelling)
			}
		})
	}
}

// An option that takes a value should say what the value is, or the synopsis reads
// `-k VALUE` and tells the reader nothing.
func TestUsage_namesTheArgumentOfEveryOptionThatTakesOne(t *testing.T) {
	for _, command := range commands {
		if command.Builtin {
			continue
		}
		for _, letter := range command.ValueShort {
			if _, ok := valuePlaceholders[command.Name+string(letter)]; !ok {
				t.Errorf("%s -%c takes a value and does not name it; add it to valuePlaceholders",
					command.Name, letter)
			}
		}
	}
}

// A placeholder for something that is not an option that takes a value is dead
// weight, and dead weight is how a table stops being trustworthy.
func TestUsage_hasNoUnusedPlaceholders(t *testing.T) {
	for key := range valuePlaceholders {
		found := false
		for _, command := range commands {
			if !strings.HasPrefix(key, command.Name) {
				continue
			}
			letter := strings.TrimPrefix(key, command.Name)
			if len(letter) == 1 && strings.Contains(command.ValueShort, letter) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("valuePlaceholders[%q] matches no option that takes a value", key)
		}
	}
}

func TestUsageFor_rendersASynopsisAndTheOptions(t *testing.T) {
	// When
	text, ok := UsageFor("sort")

	// Then
	if !ok {
		t.Fatal("sort has no usage")
	}
	for _, want := range []string{
		// The flags cluster, because that is how they are typed.
		"Usage: sort [-bfnru]",
		// The ones that take a value are spelled out, with the value named.
		"[-k KEYDEF]",
		"[-t SEP]",
		"[FILE]...",
		"Sort lines of text.",
		"-k KEYDEF",
		"sort on this key rather than the whole line",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("usage for sort is missing %q:\n%s", want, text)
		}
	}
}

func TestUsageFor_rendersLongOptions(t *testing.T) {
	// When
	text, _ := UsageFor("ls")

	// Then
	if !strings.Contains(text, "--color") {
		t.Fatalf("usage for ls does not mention --color:\n%s", text)
	}
}

func TestUsageFor_reportsNothingForAnUnknownName(t *testing.T) {
	// When
	_, ok := UsageFor("nosuchapplet")

	// Then
	if ok {
		t.Fatal("an unknown name should have no usage")
	}
}

func containsName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
