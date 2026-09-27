package runtime

import "strings"

// GLOBIGNORE, bash's: a colon-separated list of patterns, and a pathname a glob produces that
// matches one of them is left out. Setting it to anything also has a glob match names that
// begin with a dot, as dotglob does, and never `.` or `..`. A glob all of whose pathnames are
// left out matched nothing: it stays as written, or goes under nullglob. busybox has no
// GLOBIGNORE, and it was not read at all.
//
// A pattern is matched a directory at a time, and may match the leading directories alone,
// as bash matches it -- fnmatch's FNM_PATHNAME and FNM_LEADING_DIR, measured: `*.txt` keeps
// foo/two.txt, where `foo/*.txt` and `foo*` and `*` all leave it out.

// globIgnoring reports GLOBIGNORE set to something.
func (r Runtime) globIgnoring() bool {
	return r.vars["GLOBIGNORE"] != ""
}

// withoutIgnored is the pathnames GLOBIGNORE leaves.
func (r Runtime) withoutIgnored(matches []string) []string {
	if !r.globIgnoring() {
		return matches
	}
	patterns := splitGlobIgnore(r.vars["GLOBIGNORE"])
	kept := matches[:0]
	for _, match := range matches {
		if !globIgnored(match, patterns) {
			kept = append(kept, match)
		}
	}
	return kept
}

func globIgnored(path string, patterns []string) bool {
	if base := path[strings.LastIndexByte(path, '/')+1:]; base == "." || base == ".." {
		return true
	}
	components := strings.Split(path, "/")
	for _, pattern := range patterns {
		parts := strings.Split(pattern, "/")
		if pattern == "" || len(parts) > len(components) {
			continue
		}
		matched := true
		for index, part := range parts {
			if !matchShellPattern(part, components[index]) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

// splitGlobIgnore cuts GLOBIGNORE at its colons, but not at one inside a bracket expression,
// so `[[:alnum:]]*` is one pattern, and not at one escaped with a backslash.
func splitGlobIgnore(value string) []string {
	var patterns []string
	var current strings.Builder
	depth := 0
	for index := 0; index < len(value); index++ {
		switch char := value[index]; {
		case char == '\\' && index+1 < len(value):
			current.WriteByte(char)
			index++
			current.WriteByte(value[index])
			continue
		case char == '[':
			depth++
		case char == ']' && depth > 0:
			depth--
		case char == ':' && depth == 0:
			patterns = append(patterns, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(value[index])
	}
	return append(patterns, current.String())
}
