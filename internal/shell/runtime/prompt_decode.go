package runtime

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// decodePrompt renders a prompt's backslash escapes as bash's decode_prompt_string does:
//
//	\u \h \H    the user, the host to its first dot, the whole host
//	\w \W       the working directory with $HOME as ~, and its last component
//	\d \t \T \@ \A   the date, and the time in 24 and 12 hours, and HH:MM
//	\D{format}  strftime's format, and %X when it is empty
//	\s \v \V    the shell's name, and its version as major.minor and in full
//	\# \! \j \l the command's number, its history number, the jobs, the terminal
//	\$          # for root and $ for anyone else
//	\a \e \n \r \\ \NNN   a bell, an escape, a newline, a return, a backslash, a byte in octal
//	\[ \]       dropped: they mark what a line editor does not count, and this one counts
//
// Any other backslash stays as written, before the character it came with. busybox's line
// editor knows a subset -- it shows every time as HH:MM, it says of its own \v that it is
// mishandled, and the rest it does not have -- so bash's is the set.
//
// fact answers what \u \h \H \w \W \s \v \V \# \! \j \l and \$ stand for, asked only for the
// ones the text holds. quoted is ${var@P}'s form, which bash expands after decoding: what an
// escape stands for is quoted for that expansion, so a directory named `$foo` stays $foo, and
// \$ is `\$`, the expansion's own escape for it.
func decodePrompt(text string, now time.Time, fact func(byte) string, quoted bool) string {
	var out strings.Builder
	for index := 0; index < len(text); index++ {
		if text[index] != '\\' {
			out.WriteByte(text[index])
			continue
		}
		if index+1 == len(text) {
			out.WriteByte('\\')
			break
		}
		index++
		escape := text[index]
		switch escape {
		case '0', '1', '2', '3', '4', '5', '6', '7':
			value, digits := 0, 0
			for ; digits < 3 && index+digits < len(text) && text[index+digits] >= '0' && text[index+digits] <= '7'; digits++ {
				value = value*8 + int(text[index+digits]-'0')
			}
			index += digits - 1
			// A byte, as bash's char keeps it: \555 is m. \000 is none.
			if value&0xff != 0 {
				out.WriteByte(byte(value))
			}
		case 'a':
			out.WriteByte('\a')
		case 'e':
			out.WriteByte(0x1b)
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case '\\':
			out.WriteByte('\\')
		case '[', ']':
		case 'd', 't', 'T', '@', 'A':
			out.WriteString(applets.Strftime(now, promptTimeFormats[escape]))
		case 'D':
			if index+1 == len(text) || text[index+1] != '{' {
				out.WriteString(`\D`)
				continue
			}
			// Up to the closing brace, or to the end of the text when there is none.
			format, _, _ := strings.Cut(text[index+2:], "}")
			index += 2 + len(format)
			if format == "" {
				format = "%X"
			}
			out.WriteString(quotePromptValue(applets.Strftime(now, format), quoted))
		case '$':
			symbol := fact('$')
			if quoted && symbol == "$" {
				symbol = `\$`
			}
			out.WriteString(symbol)
		case 'u', 'h', 'H', 'w', 'W', 's':
			out.WriteString(quotePromptValue(visiblePromptValue(fact(escape)), quoted))
		case 'v', 'V', '#', '!', 'j', 'l':
			out.WriteString(fact(escape))
		default:
			out.WriteByte('\\')
			out.WriteByte(escape)
		}
	}
	return out.String()
}

// promptTimeFormats are the times bash gives each escape, in strftime's terms.
var promptTimeFormats = map[byte]string{
	'd': "%a %b %d", 't': "%H:%M:%S", 'T': "%I:%M:%S", '@': "%I:%M %p", 'A': "%H:%M",
}

// quotePromptValue is bash's sh_backslash_quote_for_double_quotes, when the text is to be
// expanded after it is decoded.
func quotePromptValue(value string, quoted bool) string {
	if !quoted {
		return value
	}
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`").Replace(value)
}

// visiblePromptValue writes a control character or a byte that is not UTF-8 as \xNN, so a
// directory or a host name cannot drive the terminal the prompt is drawn on.
func visiblePromptValue(value string) string {
	var out strings.Builder
	for len(value) > 0 {
		character, width := utf8.DecodeRuneInString(value)
		switch {
		case character == utf8.RuneError && width == 1:
			fmt.Fprintf(&out, `\x%02x`, value[0])
		case character < ' ' || character == 0x7f || 0x80 <= character && character <= 0x9f:
			fmt.Fprintf(&out, `\x%02x`, character)
		default:
			out.WriteString(value[:width])
		}
		value = value[width:]
	}
	return out.String()
}
