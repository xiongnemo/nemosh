package runtime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

var ErrResolvedNativePathNotAbsolute = errors.New("resolved native path is not absolute")

type PathSettings struct {
	Config  pathmodel.Config
	TmpRoot WorkingDirectory
	// AliasSpelling reports a Windows path the way MSYS does, `/c/Users/nemo`, rather than
	// busybox-w32's `C:/Users/nemo`. That was the only spelling until 2026-10-19, when
	// `nvim ~/.ssh/config` opened `\c\Users\nemo\.ssh\config` on the current drive: a native
	// program is handed the words it is given, and only a shell that is MSYS can read the alias.
	// NEMOSH_PATHS=msys is the way back to it. Either spelling resolves the same in both.
	AliasSpelling bool
}

// pathSpellingVariable names the spelling: "msys" for the drive alias, and busybox-w32's
// otherwise, unset included.
const pathSpellingVariable = "NEMOSH_PATHS"

func DefaultPathSettings() PathSettings {
	return PathSettings{Config: pathmodel.DefaultConfig(), TmpRoot: WorkingDirectory(os.TempDir())}
}

func (r Runtime) ResolveNemoshPath(path string) (pathmodel.ResolvedPath, error) {
	if r.initErr != nil {
		return pathmodel.ResolvedPath{}, r.initErr
	}
	// An empty word names nothing: POSIX resolves no null pathname, and every system call
	// refuses one with ENOENT, which the empty native path answered here keeps. The model joined
	// it to the working directory, so `rm -rf ""` emptied that, `[ -d "" ]` was true and
	// `exec 3< ""` opened it. A caller that means the working directory -- a glob's base, an
	// empty PATH entry, cd's empty operand -- says "." itself.
	if path == "" {
		return pathmodel.ResolvedPath{}, nil
	}
	resolved, err := r.paths.resolve(path)
	if err != nil {
		return pathmodel.ResolvedPath{}, err
	}
	if !resolved.Device && !filepath.IsAbs(resolved.Native) {
		return pathmodel.ResolvedPath{}, fmt.Errorf("resolved native path %q: %w: %w", resolved.Native, ErrResolvedNativePathNotAbsolute, errExternalPathNotAbsolute)
	}
	return resolved, nil
}

func (r Runtime) nativeWorkingDirectory() (string, error) {
	if r.initErr != nil {
		return "", r.initErr
	}
	return r.paths.nativeWorkingDirectory()
}

// SpellPath is a canonical path as this shell reports it, for an applet printing one.
func (r Runtime) SpellPath(path pathmodel.Path) string { return r.paths.spell(path) }

func (r Runtime) CanonicalizeNativePath(origin pathmodel.Path, native string) (pathmodel.Path, error) {
	if r.initErr != nil {
		return "", r.initErr
	}
	return r.paths.canonicalizeNativePath(origin, native)
}

// withRealCase re-canonicalizes a resolved directory under the spelling the
// filesystem stores for it. A virtual root such as /tmp or a device path has no
// on-disk name to ask about, and the drive alias itself stays lowercase, which
// is what windows-path-model.md asks for.
func (r Runtime) withRealCase(resolved pathmodel.ResolvedPath) pathmodel.ResolvedPath {
	if resolved.Device || resolved.Native == "" {
		return resolved
	}
	actual := realCaseNativePath(resolved.Native)
	if actual == resolved.Native {
		return resolved
	}
	corrected, err := r.paths.canonicalizeNativePath(resolved.Canonical, actual)
	if err != nil {
		return resolved
	}
	resolved.Canonical = corrected
	resolved.Native = actual
	return resolved
}
