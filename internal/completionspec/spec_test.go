package completionspec_test

import (
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/completionspec"
)

// The rules a contributed file is held to. Each one is a way a spec can look
// right and be wrong, which is the only failure mode that matters here: nothing
// can run the command to find out the hard way.
func TestParse_refuses(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		fragment string
	}{
		{
			// The file name is the lookup key, so a mismatch means the file
			// would never be found for the command it describes.
			name:     "a name that is not the file's",
			source:   valid(`name = "curl"`),
			fragment: "does not match the file name",
		},
		{
			name:     "no provenance",
			source:   "[command]\nname = \"adb\"\n",
			fragment: "meta.derived-from is required",
		},
		{
			name:     "no version",
			source:   "[meta]\nderived-from = \"adb --help\"\nmeasured-on = \"2026-08-16\"\n\n[command]\nname = \"adb\"\n",
			fragment: "meta.tool-version is required",
		},
		{
			// A misspelled key that is silently dropped leaves a file that looks
			// right and completes nothing.
			name:     "a key this format does not have",
			source:   valid(`name = "adb"` + "\n" + `shortt = "abc"`),
			fragment: "keys this format does not have",
		},
		{
			name:     "an operand kind that does not exist",
			source:   valid(`name = "adb"` + "\n" + `operand = "url"`),
			fragment: "is not one of path, directory, host, none",
		},
		{
			name:     "a value option that is not an option",
			source:   valid(`name = "adb"` + "\n" + `short = "ab"` + "\n" + `value-short = "z"`),
			fragment: "-z takes a value but is not in short",
		},
		{
			name:     "a file option that takes no value",
			source:   valid(`name = "adb"` + "\n" + `short = "ab"` + "\n" + `file-short = "a"`),
			fragment: "-a takes a file but not a value",
		},
		{
			name:     "a long option written with its dashes",
			source:   valid(`name = "adb"` + "\n" + `long = ["--verbose"]`),
			fragment: "written with its dashes",
		},
		{
			name:     "the same option twice",
			source:   valid(`name = "adb"` + "\n" + `short = "aa"`),
			fragment: "listed twice",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, err := completionspec.Parse("adb", []byte(test.source))

			// Then
			if err == nil || !strings.Contains(err.Error(), test.fragment) {
				t.Fatalf("Parse = %v, want an error containing %q", err, test.fragment)
			}
		})
	}
}

func TestParse_acceptsASubcommandTree(t *testing.T) {
	// Given
	source := valid(`name = "adb"`+"\n"+`operand = "none"`+"\n"+`short = "s"`+"\n"+`value-short = "s"`) + `
[[subcommand]]
name = "install"
operand = "path"
short = "rt"
`

	// When
	spec, err := completionspec.Parse("adb", []byte(source))

	// Then
	if err != nil {
		t.Fatalf("Parse = %v", err)
	}
	if len(spec.Subcommand) != 1 || spec.Subcommand[0].Name != "install" {
		t.Fatalf("subcommands = %+v, want one named install", spec.Subcommand)
	}
	if spec.Subcommand[0].Operand != completionspec.OperandPath {
		t.Fatalf("operand = %q, want path", spec.Subcommand[0].Operand)
	}
}

// A subcommand is a surface like any other, so the same rules reach into it --
// otherwise the strictness would stop exactly where the files get long.
func TestParse_holdsSubcommandsToTheSameRules(t *testing.T) {
	// Given
	source := valid(`name = "adb"`) + `
[[subcommand]]
name = "install"
short = "r"
file-short = "r"
`

	// When
	_, err := completionspec.Parse("adb", []byte(source))

	// Then
	if err == nil || !strings.Contains(err.Error(), "adb install: -r takes a file but not a value") {
		t.Fatalf("Parse = %v, want the subcommand named in the error", err)
	}
}

func valid(command string) string {
	return `[meta]
derived-from = "adb --help"
tool-version = "Android Debug Bridge version 1.0.41"
measured-on = "2026-08-16"

[command]
` + command + "\n"
}
