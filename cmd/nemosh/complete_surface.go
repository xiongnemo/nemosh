package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/xiongnemo/nemosh/completions"
	"github.com/xiongnemo/nemosh/internal/capability"
	"github.com/xiongnemo/nemosh/internal/completionspec"
)

// commandSurface is what completion knows about the word being typed, whichever
// of the two sources it came from.
//
// Two sources, one shape. internal/capability is what this shell ships, bound to
// behaviour by a test that runs each applet; completions/ is data about programs
// it does not ship and cannot run. Keeping them apart is deliberate -- a data
// file must not be able to describe `ls` and take that guarantee away -- and
// keeping the *shape* the same is what stops the completion code from caring
// which it got.
type commandSurface struct {
	operand    completionspec.OperandKind
	short      string
	valueShort string
	fileShort  string
	long       []string
	valueLong  []string
	fileLong   []string
	// subcommands is offered for the word that selects a surface, and is empty
	// for everything that has none.
	subcommands []string
}

func (s commandSurface) takesValue(flag rune) bool {
	return strings.ContainsRune(s.valueShort, flag)
}

func (s commandSurface) takesFile(flag rune) bool {
	return strings.ContainsRune(s.fileShort, flag)
}

// options is what could follow a dash, long forms written with their dashes.
func (s commandSurface) options() []string {
	offers := make([]string, 0, len(s.short)+len(s.long))
	for _, flag := range s.short {
		offers = append(offers, "-"+string(flag))
	}
	for _, name := range s.long {
		offers = append(offers, "--"+name)
	}
	return offers
}

// surfaceFor resolves the words already typed to a surface.
//
// The shipped table is asked first. A spec file may not name an applet or a
// builtin -- a test in completions/ enforces it -- so the order settles nothing
// contentious; it just means the measured answer is the one that cannot be
// shadowed.
func surfaceFor(specs *completionspec.Registry, words []string) (commandSurface, bool) {
	if len(words) == 0 {
		return commandSurface{}, false
	}
	if command, ok := capability.Lookup(words[0]); ok {
		return commandSurface{
			operand:    shippedOperandKind(command.Operand),
			short:      command.Short,
			valueShort: command.ValueShort,
			fileShort:  command.FileShort,
			long:       command.Long,
		}, true
	}
	if specs == nil {
		return commandSurface{}, false
	}
	spec, ok := specs.Lookup(words[0])
	if !ok {
		return commandSurface{}, false
	}
	selected := spec.SurfaceFor(words[1:])
	surface := commandSurface{
		operand:    selected.Operand,
		short:      selected.Short,
		valueShort: selected.ValueShort,
		fileShort:  selected.FileShort,
		long:       selected.Long,
		valueLong:  selected.ValueLong,
		fileLong:   selected.FileLong,
	}
	// Only while the subcommand itself is still the word being chosen. Once
	// `adb install` is on the line, offering `devices` again would be offering a
	// word that cannot go there.
	if selected.Name == spec.Command.Name {
		surface.subcommands = spec.SubcommandNames()
	}
	return surface, true
}

// shippedOperandKind maps the compiled-in table's vocabulary onto the spec's,
// which is the richer of the two: a spec can say `none`, and no command this
// shell ships needs to, because every one of them can be handed a path.
//
// Mapping this direction rather than the other is what keeps `adb devices ` able
// to answer "nothing goes here". Flattening `none` into "any path" lost exactly
// that, and the test for it failed the first time it ran.
func shippedOperandKind(kind capability.OperandKind) completionspec.OperandKind {
	switch kind {
	case capability.Directory:
		return completionspec.OperandDirectory
	case capability.HostName:
		return completionspec.OperandHost
	default:
		return completionspec.OperandPath
	}
}

// specSearchDirectories is where a user's own specs live, in the order they are
// tried. Theirs wins over what was compiled in, which is the whole fix for a
// bundled spec that is wrong for a particular machine.
func specSearchDirectories(lookup func(string) (string, bool)) []string {
	var dirs []string
	if appData, ok := lookup("APPDATA"); ok && appData != "" {
		dirs = append(dirs, filepath.Join(appData, "nemosh", "completions"))
	}
	if config, ok := lookup("XDG_CONFIG_HOME"); ok && config != "" {
		dirs = append(dirs, filepath.Join(config, "nemosh", "completions"))
	} else if home, ok := lookup("HOME"); ok && home != "" {
		dirs = append(dirs, filepath.Join(home, ".config", "nemosh", "completions"))
	}
	return dirs
}

func newSpecRegistry(lookup func(string) (string, bool)) *completionspec.Registry {
	return completionspec.NewRegistry(completions.Files, specSearchDirectories(lookup)...)
}

// environmentLookup is the process environment, for the editor built before a
// runtime exists.
func environmentLookup(name string) (string, bool) { return os.LookupEnv(name) }
