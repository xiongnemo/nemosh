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
	// A pattern's quoted characters are escaped (see escapeGlob); a fixed stretch is a path,
	// so it goes back to the characters it stands for.
	base := unescapeGlob(strings.Join(segments[:fixed], "/"))
	if base == "" && fixed > 0 {
		base = "/"
	}
	matches := []string{base}
	for index, segment := range segments[fixed:] {
		// `**` crosses directories, but only when asked: without globstar bash reads
		// it as an ordinary `*`, and so does this.
		if segment == "**" && r.options.globStar {
			matches = r.expandGlobStar(matches, fixed+index == len(segments)-1, index == 0)
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
	// What GLOBIGNORE leaves out is not a match; see glob_ignore.go. Nor is the current
	// directory, which `**/` reaches as nothing, and a path two `**` reach is one match:
	// `**/**/*.md` names each file once.
	matches = slices.DeleteFunc(r.withoutIgnored(matches), func(match string) bool { return match == "" })
	if len(matches) == 0 {
		return nil
	}
	slices.Sort(matches)
	return slices.Compact(matches)
}

func (r Runtime) expandPathSegment(bases []string, segment string) []string {
	var matches []string
	for _, base := range bases {
		if !containsGlobMeta(segment) {
			// A fixed segment after a globbed one still has to exist, or the
			// branch it sits on is not a match.
			candidate := joinGlobPath(base, unescapeGlob(segment))
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
		// is what keeps `*` from returning every hidden file -- unless dotglob or
		// GLOBIGNORE says otherwise.
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(segment, ".") && !r.options.dotGlob && !r.globIgnoring() {
			continue
		}
		if !r.matchGlobSegment(segment, name) || r.hiddenFromGlob(entry) {
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
		// `/dev/*` expands, which is the other half of making the devices
		// discoverable: `echo /dev/*` is how somebody finds out what is there
		// without knowing which applet to ask. A device path that is not the
		// directory has nothing to list, and an unmatched pattern is left literal
		// by the caller, which is what a shell does with one.
		entries, ok := ReadDeviceDir(string(resolved.Canonical))
		if !ok {
			return nil, os.ErrInvalid
		}
		return entries, nil
	}
	return os.ReadDir(resolved.Native)
}

func (r Runtime) pathExists(candidate string) bool {
	resolved, err := r.ResolveNemoshPath(candidate)
	if err != nil {
		return false
	}
	if resolved.Device {
		// A device exists for globbing as it does for `test -e`. Answering no here
		// made `echo /dev/null*` refuse to expand a pattern whose match was right
		// there.
		name := string(resolved.Canonical)
		if _, ok := StatDeviceDir(name); ok {
			return true
		}
		_, ok := StatDevice(name)
		return ok
	}
	_, statErr := os.Lstat(resolved.Native)
	return statErr == nil
}

// unescapeGlob is the text an escaped pattern stands for, each backslash dropped for the
// character after it.
func unescapeGlob(pattern string) string {
	if !strings.Contains(pattern, `\`) {
		return pattern
	}
	var out strings.Builder
	for index := 0; index < len(pattern); index++ {
		if pattern[index] == '\\' && index+1 < len(pattern) {
			index++
		}
		out.WriteByte(pattern[index])
	}
	return out.String()
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

// containsGlobMeta reports a pattern character in text: `*`, `?` or `[`, or the opening of an
// extended group, which makes a pattern with none of them. `rm !(keep)` and `echo
// @(foo|bar).py` were left as written, since they had no star, question mark or bracket.
func containsGlobMeta(text string) bool {
	return strings.ContainsAny(text, "*?[") || hasExtendedPattern(text)
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
// Directories only when more of the pattern follows, because what follows has to be looked
// up inside something. `**/*.go` is the shape it exists for: the bases become every directory
// in the tree and the next segment matches files in each. As the pattern's last segment it is
// bash's "all files and zero or more directories": every file beneath the base too, and the
// base itself -- spelled with its slash, `c/`, when it is the pattern's literal start, as bash
// spells it. It gave the directories alone, so `echo dir/**` listed no file. A base that is no
// directory is no match, so `x/**` stays as written.
//
// Bounded by globStarDepth. A pattern is not worth an unbounded walk of a filesystem
// that may be a network drive, and a shell that appears to hang while a user waits
// for a prompt is worse than one that misses a very deep file.
func (r Runtime) expandGlobStar(bases []string, last, literal bool) []string {
	var matches []string
	frontier := bases
	for depth := 0; depth < globStarDepth && len(frontier) > 0; depth++ {
		var next []string
		for _, base := range frontier {
			entries, err := r.readDirectory(base)
			if err != nil {
				continue
			}
			if depth == 0 {
				matches = append(matches, globStarBase(base, last, literal)...)
			}
			for _, entry := range entries {
				name := entry.Name()
				if strings.HasPrefix(name, ".") && !r.options.dotGlob || r.hiddenFromGlob(entry) {
					continue
				}
				if entry.IsDir() {
					next = append(next, joinGlobPath(base, name))
				}
				if entry.IsDir() || last {
					matches = append(matches, joinGlobPath(base, name))
				}
			}
		}
		frontier = next
	}
	return matches
}

// globStarBase is a base as `**`'s match of zero directories: the base itself, and with a
// slash when it is the literal start of a pattern that ends there, `c/**`. The current
// directory has no name, and ends a pattern as nothing.
func globStarBase(base string, last, literal bool) []string {
	switch {
	case !last:
		return []string{base}
	case base == "":
		return nil
	case literal:
		return []string{strings.TrimSuffix(base, "/") + "/"}
	}
	return []string{base}
}

// globStarDepth is how far `**` descends. Deep enough for a source tree -- the
// deepest path in this repository is six segments -- and shallow enough that a
// mistaken `**` at the root of a drive does not become a filesystem walk.
const globStarDepth = 32
