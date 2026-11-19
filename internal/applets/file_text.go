package applets

import (
	"bytes"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

// What libmagic's ascmagic.c and encoding.c say of text: its encoding, a script's interpreter from
// its #!, and the line terminators, long lines, escapes and overstriking it has.

// describeText is a file that matched no magic: text of an encoding, or data.
func describeText(head []byte) (string, string) {
	encoding, charset, body := fileEncoding(head)
	if encoding == "" {
		return "data", "application/octet-stream; charset=binary"
	}
	description, mime := encoding+" text", "text/plain; charset="+charset
	if bytes.HasPrefix(body, []byte("#!")) {
		script, scriptMime := fileScript(body)
		description, mime = script+", "+encoding+" text executable", scriptMime+"; charset="+charset
	}
	return description + fileTextDetails(body), mime
}

// fileEncoding is the text's encoding as ascmagic names it, its charset, and the text as UTF-8 or
// bytes for what follows to look at; an empty name for what is not text.
func fileEncoding(head []byte) (string, string, []byte) {
	switch {
	case bytes.HasPrefix(head, []byte{0xef, 0xbb, 0xbf}) && utf8.Valid(head[3:]):
		return "Unicode text, UTF-8 (with BOM)", "utf-8", head[3:]
	case len(head) >= 2 && (head[0] == 0xff && head[1] == 0xfe || head[0] == 0xfe && head[1] == 0xff):
		little := head[0] == 0xff
		var text []byte
		for index := 2; index+1 < len(head); index += 2 {
			unit := rune(head[index]) | rune(head[index+1])<<8
			if !little {
				unit = rune(head[index])<<8 | rune(head[index+1])
			}
			text = utf8.AppendRune(text, unit)
		}
		if little {
			return "Unicode text, UTF-16, little-endian", "utf-16le", text
		}
		return "Unicode text, UTF-16, big-endian", "utf-16be", text
	}
	ascii, high := true, false
	for _, character := range head {
		switch {
		case character >= 0x80:
			ascii, high = false, true
		case character < 0x20 && !strings.ContainsRune("\a\b\t\n\v\f\r\x1b", rune(character)), character == 0x7f:
			return "", "", nil
		}
	}
	switch {
	case ascii:
		return "ASCII", "us-ascii", head
	case utf8.Valid(head):
		return "Unicode text, UTF-8", "utf-8", head
	case high:
		for _, character := range head {
			if character >= 0x80 && character < 0xa0 {
				return "Non-ISO extended-ASCII", "unknown-8bit", head
			}
		}
		return "ISO-8859", "iso-8859-1", head
	}
	return "", "", nil
}

// fileScript is a script's name from its #! line, as libmagic's commands magic gives it, and its
// MIME type: the interpreter, or the program env runs.
func fileScript(text []byte) (string, string) {
	line, _, _ := bytes.Cut(text[2:], []byte("\n"))
	fields := strings.Fields(strings.TrimSpace(string(line)))
	if len(fields) == 0 {
		return "a  script", "text/plain"
	}
	interpreter := fields[0]
	if path.Base(interpreter) == "env" && len(fields) > 1 {
		interpreter = fields[1]
	}
	switch name := path.Base(interpreter); {
	case name == "sh" || name == "dash":
		return "POSIX shell script", "text/x-shellscript"
	case name == "bash":
		return "Bourne-Again shell script", "text/x-shellscript"
	case name == "zsh":
		return "Paul Falstad's zsh script", "text/x-shellscript"
	case strings.HasPrefix(name, "python"):
		return "Python script", "text/x-script.python"
	case name == "perl":
		return "Perl script", "text/x-perl"
	case name == "node" || name == "nodejs":
		return "Node.js script", "application/javascript"
	case name == "ruby":
		return "Ruby script", "text/x-ruby"
	case name == "awk" || name == "gawk":
		return "awk or perl script", "text/x-awk"
	}
	return "a " + strings.Join(fields, " ") + " script", "text/plain"
}

// fileTextDetails are ascmagic's clauses after the encoding: very long lines, the line terminators
// unless they are all LF, escape sequences, overstriking.
func fileTextDetails(text []byte) string {
	var crlf, cr, lf, longest, line int
	escapes, backspaces := false, false
	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '\r':
			if index+1 < len(text) && text[index+1] == '\n' {
				crlf++
				index++
			} else {
				cr++
			}
			line = 0
			continue
		case '\n':
			lf++
			line = 0
			continue
		case 0x1b:
			escapes = true
		case '\b':
			backspaces = true
		}
		line++
		longest = max(longest, line)
	}
	var details strings.Builder
	if longest > 300 {
		fmt.Fprintf(&details, ", with very long lines (%d)", longest)
	}
	if crlf+cr+lf == 0 {
		details.WriteString(", with no line terminators")
	} else if crlf > 0 || cr > 0 {
		var kinds []string
		for _, kind := range []struct {
			name  string
			count int
		}{{"CRLF", crlf}, {"CR", cr}, {"LF", lf}} {
			if kind.count > 0 {
				kinds = append(kinds, kind.name)
			}
		}
		details.WriteString(", with " + strings.Join(kinds, ", ") + " line terminators")
	}
	if escapes {
		details.WriteString(", with escape sequences")
	}
	if backspaces {
		details.WriteString(", with overstriking")
	}
	return details.String()
}
