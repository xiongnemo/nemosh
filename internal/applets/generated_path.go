package applets

import (
	"path/filepath"

	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

// canonicalizeGeneratedPath is a path an applet worked out, realpath's or readlink -f's, in
// the shell's own spelling.
func canonicalizeGeneratedPath(view ProcessView, origin pathmodel.Path, native string) string {
	if resolver, ok := view.(interface {
		CanonicalizeNativePath(pathmodel.Path, string) (pathmodel.Path, error)
	}); ok {
		if canonical, err := resolver.CanonicalizeNativePath(origin, native); err == nil {
			return spellPath(view, canonical)
		}
	}
	return filepath.ToSlash(native)
}

// spellPath is a canonical path as the shell reports it -- `C:/Users/nemo` on Windows, as
// busybox-w32's realpath prints it, or `/c/Users/nemo` under NEMOSH_PATHS=msys -- so that
// `nvim "$(realpath x)"` hands nvim a path it can open, and `pwd` and realpath agree.
func spellPath(view ProcessView, path pathmodel.Path) string {
	if speller, ok := view.(interface{ SpellPath(pathmodel.Path) string }); ok {
		return speller.SpellPath(path)
	}
	return string(path)
}
