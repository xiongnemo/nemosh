// Package completionspec is the format for describing a command this shell does
// not ship, so Tab can complete it.
//
// It is data, not code. bash-completion registers shell functions and runs them
// on every Tab; zsh autoloads `_cmd` functions; fish sources declarative
// `complete` lines. The first of those is out of reach here for a reason that is
// not preference: this shell is one static binary, and executing a shell script
// per keystroke would need the shell to be good enough to host bash-completion's
// code and would break the rule that nothing on the suggestion path runs
// anything. So the format is a table, and the loader reads it.
//
// What a spec may say is deliberately narrower than what those three can express.
// There is no branching, no code, and nothing that runs -- a later phase adds a
// declared external source for candidates (`adb devices` for a serial number),
// and even that will be an argument vector rather than a shell string, so a spec
// cannot smuggle a second command past the reader.
package completionspec

import "fmt"

// OperandKind is what a non-option word of this command can be.
type OperandKind string

const (
	// OperandPath is any file or directory: the ordinary answer.
	OperandPath OperandKind = "path"
	// OperandDirectory is a directory only, for a command a file could never
	// have been meant for.
	OperandDirectory OperandKind = "directory"
	// OperandHost is a remote machine, from the host index.
	OperandHost OperandKind = "host"
	// OperandNone is a command that takes no operand at all. Offering paths
	// there is not helpful, it is noise from the wrong universe.
	OperandNone OperandKind = "none"
)

// Spec is one command's file.
type Spec struct {
	Meta       Meta         `toml:"meta"`
	Command    Command      `toml:"command"`
	Subcommand []Subcommand `toml:"subcommand"`
}

// Meta is where the claims came from.
//
// Required, and required to be specific. Nothing in CI can run `adb` to check a
// word of this file, so the only defence against it quietly rotting is that it
// says which version of which program it was read off and when. The ssh row this
// format replaces carried the same three facts in a comment; here they are
// fields, so a test can insist on them.
type Meta struct {
	// DerivedFrom is the command that produced the text this was written from --
	// `adb --help`, not "the man page" or "the internet".
	DerivedFrom string `toml:"derived-from"`
	// ToolVersion is what that program called itself. It matters more than it
	// looks: `wget` on the machine this was written on is busybox's applet, not
	// GNU wget, and the two have different options under one name.
	ToolVersion string `toml:"tool-version"`
	// MeasuredOn is the date, so a reader can weigh how stale it may be.
	MeasuredOn string `toml:"measured-on"`
	// GeneratedBy names the script, where one wrote the file. Optional, and it
	// says something a reader wants: a generated spec is exactly as good as the
	// help text it read, while a hand-written one carries a judgement about what
	// was worth claiming.
	GeneratedBy string `toml:"generated-by"`
}

// Command is the surface of the command itself.
type Command struct {
	Name    string      `toml:"name"`
	Operand OperandKind `toml:"operand"`
	// Short is every accepted option letter as one string, because `-al`
	// clusters and the letter is the unit.
	Short string `toml:"short"`
	// ValueShort is the subset that consumes the following word. Completion
	// needs it to know that the word after `-s` is a serial and not an operand.
	ValueShort string `toml:"value-short"`
	// FileShort is the subset of ValueShort whose consumed word is a path.
	FileShort string `toml:"file-short"`
	// Long is the accepted long option names, written without their dashes.
	Long []string `toml:"long"`
	// ValueLong and FileLong are the same two subsets for the long forms.
	ValueLong []string `toml:"value-long"`
	FileLong  []string `toml:"file-long"`
}

// Subcommand is a word that selects a different surface: `adb install` takes
// options `adb` does not, and an operand of a different kind.
type Subcommand struct {
	Name    string      `toml:"name"`
	Operand OperandKind `toml:"operand"`
	Short   string      `toml:"short"`

	ValueShort string   `toml:"value-short"`
	FileShort  string   `toml:"file-short"`
	Long       []string `toml:"long"`
	ValueLong  []string `toml:"value-long"`
	FileLong   []string `toml:"file-long"`
}

// Validate reports what is wrong with a spec, if anything.
//
// Strict, and deliberately so. A spec that is half understood is worse than one
// that is refused: a mistyped key that is ignored leaves a file that looks
// right, completes nothing, and gives the reader no reason why.
func (s Spec) Validate(name string) error {
	if s.Command.Name != name {
		return fmt.Errorf("command name %q does not match the file name %q: the file name is what the loader looks up", s.Command.Name, name)
	}
	if err := s.Meta.validate(); err != nil {
		return err
	}
	if err := validateSurface(s.Command.Name, s.Command.Operand, s.Command.Short,
		s.Command.ValueShort, s.Command.FileShort, s.Command.Long, s.Command.ValueLong, s.Command.FileLong); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, sub := range s.Subcommand {
		if sub.Name == "" {
			return fmt.Errorf("%s: a subcommand with no name", name)
		}
		if seen[sub.Name] {
			return fmt.Errorf("%s: subcommand %q appears twice", name, sub.Name)
		}
		seen[sub.Name] = true
		if err := validateSurface(name+" "+sub.Name, sub.Operand, sub.Short,
			sub.ValueShort, sub.FileShort, sub.Long, sub.ValueLong, sub.FileLong); err != nil {
			return err
		}
	}
	return nil
}

func (m Meta) validate() error {
	switch {
	case m.DerivedFrom == "":
		return fmt.Errorf("meta.derived-from is required: nothing here can run the command to check these claims, so the file has to say what it was read off")
	case m.ToolVersion == "":
		return fmt.Errorf("meta.tool-version is required: one name can be two programs, and `wget` is already both")
	case m.MeasuredOn == "":
		return fmt.Errorf("meta.measured-on is required: a claim with no date cannot be weighed for staleness")
	}
	return nil
}
