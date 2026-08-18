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
		// `**` crosses directories, but only when asked: without globstar bash reads
		// it as an ordinary `*`, and so does this.
		if segment == "**" && r.options.globStar {
			matches = r.expandGlobStar(matches)
			if len(matches) == 0 {
				return nil
			}
			continue
		}
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
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(segment, ".") && !r.options.dotGlob {
			continue
		}
		if !r.matchGlobSegment(segment, name) {
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

// matchGlobSegment applies `set -o nocaseglob`, which busybox implements as
// FNM_CASEFOLD (shell/ash.c:9230). Folding both sides rather than the pattern
// alone is what makes it work in either direction: `upper.*` finds UPPER.TXT
// and `LOWER.*` finds lower.txt.
func (r Runtime) matchGlobSegment(segment, name string) bool {
	if r.options != nil && r.options.noCaseGlob {
		return matchShellPattern(strings.ToLower(segment), strings.ToLower(name))
	}
	return matchShellPattern(segment, name)
}

// expandGlobStar answers a `**` segment: each base, and every directory beneath it.
//
// Directories only, because `**` is a path segment and what follows it has to be
// looked up inside something. `**/*.go` is the shape it exists for: the bases become
// every directory in the tree and the next segment matches files in each.
//
// Bounded by globStarDepth. A pattern is not worth an unbounded walk of a filesystem
// that may be a network drive, and a shell that appears to hang while a user waits
// for a prompt is worse than one that misses a very deep file.
func (r Runtime) expandGlobStar(bases []string) []string {
	matches := append([]string(nil), bases...)
	frontier := append([]string(nil), bases...)
	for depth := 0; depth < globStarDepth && len(frontier) > 0; depth++ {
		var next []string
		for _, base := range frontier {
			entries, err := r.readDirectory(base)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				name := entry.Name()
				if strings.HasPrefix(name, ".") && !r.options.dotGlob {
					continue
				}
				next = append(next, joinGlobPath(base, name))
			}
		}
		matches = append(matches, next...)
		frontier = next
	}
	return matches
}

// globStarDepth is how far `**` descends. Deep enough for a source tree -- the
// deepest path in this repository is six segments -- and shallow enough that a
// mistaken `**` at the root of a drive does not become a filesystem walk.
const globStarDepth = 32
