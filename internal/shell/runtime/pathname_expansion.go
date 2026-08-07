package runtime

import (
	"os"
	"slices"
	"strings"
)

// expandPathnames is POSIX 2.6.6: a field carrying an unquoted `*`, `?`, or `[`
// is replaced by the sorted pathnames it matches. Nothing did this, so `ls
// *.txt` handed the literal three characters `*.t` -- well, the literal
// `*.txt` -- to ls, which reported it as a missing file.
//
// A pattern that matches nothing is left exactly as written, which is what
// POSIX requires of a shell without nullglob, and `set -f` turns the whole
// thing off.
func (r Runtime) expandPathnames(field string) []string {
	if r.options.noGlob {
		return nil
	}
	segments := strings.Split(field, "/")
	// Everything up to the first segment with a metacharacter in it is fixed,
	// and joining it back keeps a leading `/` or a `C:` where it was.
	fixed := 0
	for fixed < len(segments) && !containsGlobMeta(segments[fixed]) {
		fixed++
	}
	if fixed == len(segments) {
		return nil
	}
	base := strings.Join(segments[:fixed], "/")
	if base == "" && fixed > 0 {
		base = "/"
	}
	matches := []string{base}
	for _, segment := range segments[fixed:] {
		matches = r.expandPathSegment(matches, segment)
		if len(matches) == 0 {
			return nil
		}
	}
	slices.Sort(matches)
	return matches
}

func (r Runtime) expandPathSegment(bases []string, segment string) []string {
	var matches []string
	for _, base := range bases {
		if !containsGlobMeta(segment) {
			// A fixed segment after a globbed one still has to exist, or the
			// branch it sits on is not a match.
			candidate := joinGlobPath(base, segment)
			if r.pathExists(candidate) {
				matches = append(matches, candidate)
			}
			continue
		}
		matches = append(matches, r.globChildren(base, segment)...)
	}
	return matches
}

func (r Runtime) globChildren(base, segment string) []string {
	entries, err := r.readDirectory(base)
	if err != nil {
		return nil
	}
	var matched []string
	for _, entry := range entries {
		name := entry.Name()
		// A leading dot is matched only by a pattern that spells one out, which
		// is what keeps `*` from returning every hidden file.
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(segment, ".") {
			continue
		}
		if !matchShellPattern(segment, name) {
			continue
		}
		matched = append(matched, joinGlobPath(base, name))
	}
	return matched
}

func (r Runtime) readDirectory(base string) ([]os.DirEntry, error) {
	if base == "" {
		base = "."
	}
	resolved, err := r.ResolveNemoshPath(base)
	if err != nil {
		return nil, err
	}
	if resolved.Device {
		return nil, os.ErrInvalid
	}
	return os.ReadDir(resolved.Native)
}

func (r Runtime) pathExists(candidate string) bool {
	resolved, err := r.ResolveNemoshPath(candidate)
	if err != nil || resolved.Device {
		return false
	}
	_, statErr := os.Lstat(resolved.Native)
	return statErr == nil
}

func joinGlobPath(base, name string) string {
	switch {
	case base == "":
		return name
	case strings.HasSuffix(base, "/"):
		return base + name
	default:
		return base + "/" + name
	}
}

func containsGlobMeta(text string) bool {
	return strings.ContainsAny(text, "*?[")
}
