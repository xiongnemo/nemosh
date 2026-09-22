package runtime

// POSIX 2.4's reserved words, in one place, and the two questions anything outside the
// parser needs to ask about them.
//
// They were scattered: `fi`, `done` and `esac` appear as a literal trio in five files, and
// `commandIntroducers` sat beside the brace rule that first needed it. That is tolerable
// inside the parser, where each site is about one construct. It stops being tolerable once
// the *line editor* needs the same answers -- it draws `case` in a colour and decides where
// a command begins, and a second opinion about the grammar is how the editor and the shell
// come to disagree about the line on screen.
//
// So the parser owns the grammar and answers for it. The editor asks rather than keeping a
// list; see cmd/nemosh/highlight.go, which had `case` painted as a command that does not
// exist.

// reservedWords is the sixteen words of POSIX 2.4. `[[` and `select` are not among them --
// POSIX has neither -- and are listed separately where they are implemented.
var reservedWords = map[string]bool{
	"!": true, "{": true, "}": true,
	"case": true, "do": true, "done": true, "elif": true, "else": true,
	"esac": true, "fi": true, "for": true, "if": true, "in": true,
	"then": true, "until": true, "while": true,
}

// commandIntroducers are the reserved words a command may follow directly, so a `{` after
// one of them opens a group -- `if { true; }; then`, `then { echo a; }`, `! { false; }` --
// and a word after one of them is a command name rather than an argument.
//
// The reserved words that are *not* here are as interesting as the ones that are. The
// closers `fi`, `done`, `esac` and `}` end a construct, so what follows them is a new
// command only by way of a separator -- `{ echo a; } echo b` is not a thing. `in` is
// followed by words or patterns. `case` and `for` are followed by a word, not a command:
// `case` by the subject, `for` by the variable.
var commandIntroducers = map[string]bool{
	"if": true, "then": true, "elif": true, "else": true,
	"while": true, "until": true, "do": true, "!": true,
	"{": true,
}

// ReservedWord reports whether word is one of the shell's reserved words, so that something
// drawing a line can say so rather than calling it a command that does not exist.
//
// `select` is included: this shell implements it, and to a reader of the line it is a
// keyword whatever POSIX calls it.
func ReservedWord(word string) bool {
	return reservedWords[word] || word == "select"
}

// CommandFollows reports whether a command may begin immediately after word.
//
// The question the line editor asks about the word before the cursor, and the question the
// brace rule asks about the word before a `{`. One answer, because two would drift: the
// editor would colour `echo` in `then echo x` as an argument while the shell ran it as a
// command.
func CommandFollows(word string) bool {
	if commandIntroducers[word] {
		return true
	}
	switch word {
	// The operators a command follows. `)` is here for a case arm's pattern and for a
	// subshell's closer, and it wants no telling apart: a command may begin after either.
	case ";", ";;", ";&", ";;&", "&", "&&", "|", "||", "(", ")", "\n":
		return true
	}
	return false
}
