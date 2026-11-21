package runtime

import (
	"errors"
	"strconv"
	"strings"
)

// Key sequences, as readline writes them in a `bind` argument and in ~/.inputrc: \C-x is a
// control key, \M-x the same key after Escape (readline's convert-meta, on in bash), \e
// Escape, the C escapes and \d for Delete, \nnn in octal and \xHH in hex. printKeyseq writes
// them back as bash's `bind -p` does: "\C-a", "\ey", "\C-?". busybox has no bind.

var errEmptyKeyseq = errors.New("an empty key sequence")

// parseKeyseq is the bytes a key sequence written in readline's notation stands for.
func parseKeyseq(text string) ([]byte, error) {
	var keys []byte
	for index := 0; index < len(text); {
		unit, next, err := keyseqUnit(text, index)
		if err != nil {
			return nil, err
		}
		keys, index = append(keys, unit...), next
	}
	if len(keys) == 0 {
		return nil, errEmptyKeyseq
	}
	return keys, nil
}

// keyseqUnit reads one key at index: a character, an escape, or \C- or \M- before one.
func keyseqUnit(text string, index int) ([]byte, int, error) {
	if text[index] != '\\' || index+1 == len(text) {
		return []byte{text[index]}, index + 1, nil
	}
	for _, prefix := range [...]string{`\C-`, `\M-`} {
		if !strings.HasPrefix(text[index:], prefix) {
			continue
		}
		if index+3 == len(text) {
			return nil, 0, errors.New("a key sequence ends in " + prefix)
		}
		unit, next, err := keyseqUnit(text, index+3)
		if err != nil {
			return nil, 0, err
		}
		if prefix == `\M-` {
			return append([]byte{0x1b}, unit...), next, nil
		}
		unit[len(unit)-1] = controlKey(unit[len(unit)-1])
		return unit, next, nil
	}
	escape := text[index+1]
	if value := strings.IndexByte("abdefnrtv", escape); value >= 0 {
		return []byte{"\a\b\x7f\x1b\f\n\r\t\v"[value]}, index + 2, nil
	}
	switch {
	case escape >= '0' && escape <= '7':
		return keyseqNumber(text, index+1, 3, 8, func(c byte) bool { return c >= '0' && c <= '7' })
	case escape == 'x' && index+2 < len(text) && isHexDigit(text[index+2]):
		return keyseqNumber(text, index+2, 2, 16, isHexDigit)
	}
	return []byte{escape}, index + 2, nil
}

// keyseqNumber reads up to width digits of a base from start as one byte.
func keyseqNumber(text string, start, width, base int, digit func(byte) bool) ([]byte, int, error) {
	end := start
	for end < len(text) && end-start < width && digit(text[end]) {
		end++
	}
	value, err := strconv.ParseUint(text[start:end], base, 8)
	if err != nil {
		return nil, 0, err
	}
	return []byte{byte(value)}, end, nil
}

func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c|0x20 >= 'a' && c|0x20 <= 'f'
}

// controlKey is the key a control chord makes of c: ? is Delete, and anything else its low five
// bits, so \C-a and \C-A are both 1.
func controlKey(c byte) byte {
	if c == '?' {
		return 0x7f
	}
	return c & 0x1f
}

// printKeyseq writes keys as bash's bind -p does.
func printKeyseq(keys []byte) string {
	var out strings.Builder
	for _, c := range keys {
		switch {
		case c == 0x1b:
			out.WriteString(`\e`)
		case c == 0x7f:
			out.WriteString(`\C-?`)
		case c < 0x20:
			name := c + 0x40
			if name >= 'A' && name <= 'Z' {
				name |= 0x20
			}
			out.WriteString(`\C-` + string(rune(name)))
		case c == '"' || c == '\\':
			out.WriteString(`\` + string(rune(c)))
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

// keyNamed is the key an inputrc's unquoted name stands for: Control- or C- and Meta- or M-
// before a character or one of readline's names for a key with none.
func keyNamed(name string) ([]byte, bool) {
	control, meta := false, false
	for {
		lower := strings.ToLower(name)
		switch {
		case strings.HasPrefix(lower, "control-"):
			control, name = true, name[len("control-"):]
		case strings.HasPrefix(lower, "c-") && len(name) > 2:
			control, name = true, name[2:]
		case strings.HasPrefix(lower, "meta-"):
			meta, name = true, name[len("meta-"):]
		case strings.HasPrefix(lower, "m-") && len(name) > 2:
			meta, name = true, name[2:]
		default:
			key, ok := namedKey(name)
			if !ok {
				return nil, false
			}
			if control {
				key = controlKey(key)
			}
			if meta {
				return []byte{0x1b, key}, true
			}
			return []byte{key}, true
		}
	}
}

// namedKey is one key by readline's name for it, or a single character.
func namedKey(name string) (byte, bool) {
	switch strings.ToLower(name) {
	case "del", "rubout":
		return 0x7f, true
	case "esc", "escape":
		return 0x1b, true
	case "lfd", "newline":
		return '\n', true
	case "ret", "return":
		return '\r', true
	case "space", "spc":
		return ' ', true
	case "tab":
		return '\t', true
	}
	if len(name) == 1 {
		return name[0], true
	}
	return 0, false
}
