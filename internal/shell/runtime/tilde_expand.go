package runtime

import "strings"

// Tilde expansion (POSIX 2.6.1), done on a word's parts before anything else in it expands.
//
// It was done on the finished field instead: the first field's leading `~` was swapped for
// HOME after the whole word had expanded. That could not see where the characters came
// from, so it had to stop at the one case it could be sure of, and three rules both
// references keep were missing:
//
//   - In an assignment a tilde-prefix begins after the `=` and after every unquoted `:`, so
//     `PATH=~/bin:~/sbin` and `x=a:~` expand every one. Only a leading `~` did.
//   - The prefix ends at the first unquoted `/`, or `:` in an assignment, and must end in the
//     text it began in: `x=~$y` with y=/a is `~/a` in both, as a user name would be, where it
//     became HOME followed by /a.
//   - HOME set to the empty string is what `~` expands to. It fell back to USERPROFILE, which
//     is right only when HOME is not set at all, so `[[ ~ ]]` was true with HOME=''.
//
// The directory an expansion gives is quoted, so it is neither split nor matched as a pattern,
// and HOME is used as it stands: with HOME=/h/, `~/x` is /h//x in both.

// tildeParts is the word's parts with each tilde-prefix replaced by the directory it names.
func (r Runtime) tildeParts(item word) []wordPart {
	if !item.expandTilde && !item.assignmentTilde && !item.valueTilde {
		return item.parts
	}
	colons := item.assignmentTilde || item.valueTilde
	var parts []wordPart
	for index, part := range item.parts {
		if part.kind != wordPartLiteral || part.quote != quoteUnquoted {
			parts = append(parts, part)
			continue
		}
		leading := index == 0 && (item.expandTilde || item.valueTilde)
		equals := index == 0 && item.assignmentTilde
		last := index == len(item.parts)-1
		parts = append(parts, r.tildeLiteral(part.text, leading, equals, colons, last)...)
	}
	return parts
}

// tildeLiteral expands the tilde-prefixes in one unquoted literal part. leading is whether a
// prefix may begin at its start, equals whether one may begin after its first `=` -- an
// assignment's -- colons whether one may begin after each `:`, and last whether a prefix may
// run to the end of the part.
func (r Runtime) tildeLiteral(text string, leading, equals, colons, last bool) []wordPart {
	var parts []wordPart
	literal := func(value string) {
		if value != "" {
			parts = append(parts, wordPart{kind: wordPartLiteral, text: value, quote: quoteUnquoted})
		}
	}
	start := 0
	for index := 0; index < len(text); index++ {
		begins := index == 0 && leading
		if index > 0 {
			begins = colons && text[index-1] == ':' || equals && text[index-1] == '=' && !strings.Contains(text[:index-1], "=")
		}
		if !begins || text[index] != '~' {
			continue
		}
		directory, width, ok := r.tildePrefix(text[index:], colons, last)
		if !ok {
			continue
		}
		literal(text[start:index])
		parts = append(parts, wordPart{kind: wordPartLiteral, text: directory, quote: quoteDouble})
		start = index + width
		index = start - 1
	}
	literal(text[start:])
	return parts
}

// tildePrefix reads the tilde-prefix at the start of text: the directory it names, and how
// long it is. It ends at the first `/`, or `:` in an assignment; one that reaches the end of
// the text ends there only when the word does too. `~` alone is the home directory and the
// stack forms are directoryTildeTarget's. A user name is left as written, as support-matrix.md
// records.
func (r Runtime) tildePrefix(text string, assignment, last bool) (string, int, bool) {
	stops := "/"
	if assignment {
		stops = "/:"
	}
	end := strings.IndexAny(text, stops)
	if end < 0 {
		if !last {
			return "", 0, false
		}
		end = len(text)
	}
	reference := text[1:end]
	if reference == "" {
		return r.homeDirectory(), end, true
	}
	directory, ok := r.directoryTildeTarget(reference)
	return directory, end, ok
}

// homeDirectory is what `~` names: HOME, even empty, or USERPROFILE when HOME is not set.
func (r Runtime) homeDirectory() string {
	if home, set := r.vars["HOME"]; set {
		return home
	}
	return r.vars["USERPROFILE"]
}
