package shellquote

import "strings"

// Ash is a value as the ash family writes one back out -- set, alias, export -p, readonly -p,
// trap and a trace -- busybox's and dash's alike: in single quotes, and each run of quotes in
// it in double quotes between them, `'it'"'"'s'`. A lone quote is an empty pair of single
// quotes and then `"'"`, and the empty value that pair alone. bash ends the quotes, escapes
// the quote and opens them again instead, and each reads back as the other.
func Ash(value string) string {
	var out strings.Builder
	for {
		quote := strings.IndexByte(value, '\'')
		if quote < 0 {
			quote = len(value)
		}
		out.WriteString("'" + value[:quote] + "'")
		if value = value[quote:]; value == "" {
			return out.String()
		}
		run := len(value) - len(strings.TrimLeft(value, "'"))
		out.WriteString(`"` + value[:run] + `"`)
		if value = value[run:]; value == "" {
			return out.String()
		}
	}
}
