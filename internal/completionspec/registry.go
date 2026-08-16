package completionspec

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Registry finds the spec for a command, if there is one.
//
// Nothing is read until the first Tab for that command, and nothing scans a
// directory: the file name is the key, so a lookup is one stat and, at most, one
// open. That is fish's arrangement and bash-completion's, and it is why a shell
// with a thousand specs available starts no slower than one with none.
//
// A user file wins over a bundled one. It is the whole fix for a bundled spec
// that is wrong for a particular machine -- and that is not hypothetical: `wget`
// is busybox's applet on one machine and GNU wget on the next.
type Registry struct {
	mu       sync.RWMutex
	dirs     []string
	bundled  fs.FS
	resolved map[string]entry
}

// entry is what a lookup found, including *not finding* anything: an absent spec
// is a fact worth caching, or every Tab for `ls` would stat two directories
// again.
type entry struct {
	spec        Spec
	found       bool
	fingerprint string
}

func NewRegistry(bundled fs.FS, dirs ...string) *Registry {
	return &Registry{dirs: dirs, bundled: bundled, resolved: map[string]entry{}}
}

// Lookup returns the spec for a command name.
//
// The cache is invalidated by the user file's size and modification time, which
// matters for the person *writing* a spec: change a line, press Tab, see it.
// A format nobody can iterate on is a format nobody contributes to.
func (r *Registry) Lookup(name string) (Spec, bool) {
	if name == "" || strings.ContainsAny(name, `/\`) {
		// A path is not a command name, and a spec called `../../etc` is not a
		// file this should be opening.
		return Spec{}, false
	}
	fingerprint := r.fingerprint(name)
	r.mu.RLock()
	cached, ok := r.resolved[name]
	r.mu.RUnlock()
	if ok && cached.fingerprint == fingerprint {
		return cached.spec, cached.found
	}
	spec, found := r.read(name)
	r.mu.Lock()
	r.resolved[name] = entry{spec: spec, found: found, fingerprint: fingerprint}
	r.mu.Unlock()
	return spec, found
}

// fingerprint is the state of the user files this name could come from. The
// bundled set cannot change under a running process, so it is not part of it.
func (r *Registry) fingerprint(name string) string {
	var state strings.Builder
	for _, dir := range r.dirs {
		info, err := os.Stat(filepath.Join(dir, name+".toml"))
		if err != nil {
			state.WriteString("-")
			continue
		}
		state.WriteString(info.ModTime().UTC().Format("20060102150405.000000000"))
		state.WriteByte(':')
		state.WriteString(strconv.FormatInt(info.Size(), 10))
		state.WriteByte(';')
	}
	return state.String()
}

// read tries the user directories in order and then what was compiled in.
//
// A spec that does not parse is treated as absent rather than as an error. This
// runs on a keystroke, and there is nowhere to report to that would not paint
// over the line being edited; the same file fails loudly in the test that walks
// the bundled directory, and a user's own file fails loudly the moment they run
// the validator over it.
func (r *Registry) read(name string) (Spec, bool) {
	for _, dir := range r.dirs {
		source, err := os.ReadFile(filepath.Join(dir, name+".toml"))
		if err != nil {
			continue
		}
		if spec, err := Parse(name, source); err == nil {
			return spec, true
		}
		return Spec{}, false
	}
	if r.bundled == nil {
		return Spec{}, false
	}
	source, err := fs.ReadFile(r.bundled, name+".toml")
	if err != nil {
		return Spec{}, false
	}
	spec, err := Parse(name, source)
	return spec, err == nil
}

// SurfaceFor picks the command's own surface, or a subcommand's when one has
// been typed.
//
// `adb install` takes options `adb` does not and an operand of a different kind,
// so the words already on the line decide which surface is being completed. The
// subcommand is the first word that is not an option and is not an option's
// value -- the same walk the operand scan makes, for the same reason.
func (s Spec) SurfaceFor(words []string) Command {
	surface := s.Command
	for index := 0; index < len(words); index++ {
		word := words[index]
		if len(word) == 2 && word[0] == '-' && strings.ContainsRune(surface.ValueShort, rune(word[1])) {
			index++
			continue
		}
		if strings.HasPrefix(word, "-") {
			continue
		}
		for _, sub := range s.Subcommand {
			if sub.Name == word {
				return Command{
					Name: sub.Name, Operand: sub.Operand, Short: sub.Short,
					ValueShort: sub.ValueShort, FileShort: sub.FileShort,
					Long: sub.Long, ValueLong: sub.ValueLong, FileLong: sub.FileLong,
				}
			}
		}
		break
	}
	return surface
}

// SubcommandNames is what to offer for the word that selects a surface.
func (s Spec) SubcommandNames() []string {
	names := make([]string, 0, len(s.Subcommand))
	for _, sub := range s.Subcommand {
		names = append(names, sub.Name)
	}
	return names
}
