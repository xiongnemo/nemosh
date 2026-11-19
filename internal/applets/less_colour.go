package applets

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// less -R, GNU's: colour drawn as colour. busybox's -R takes the sequences out as it reads
// (miscutils/less.c:550-566), so `git log --color | less -R` and `ls --color=always | less -R`
// were monochrome; GNU's passes them to the terminal, and the colours are what -R is typed for.
//
// An SGR sequence, ESC [ then digits, semicolons or colons, then m, sets the style of what
// follows and takes no column; drawLine reads it. Any other escape -- a title, a cursor move,
// a clear -- is drawn as the controls it is made of, as it is without -R, as GNU's -R passes
// colour and nothing else. A search under -R matches the text without its colours, so a word a
// colour change splits is still found, and highlighted in the colours it has.

// lessSGRAt is the SGR sequence text starts with: its length and its parameters, or 0.
func lessSGRAt(text string) (int, string) {
	if !strings.HasPrefix(text, "\x1b[") {
		return 0, ""
	}
	end := 2
	for end < len(text) && (text[end] >= '0' && text[end] <= '9' || text[end] == ';' || text[end] == ':') {
		end++
	}
	if end >= len(text) || text[end] != 'm' {
		return 0, ""
	}
	return end + 1, text[2:end]
}

// lessWithoutSGR is a line without the sequences -R draws as colour, which is what a search under
// -R reads.
func lessWithoutSGR(line string) string {
	if strings.IndexByte(line, 0x1b) < 0 {
		return line
	}
	var out strings.Builder
	for index := 0; index < len(line); {
		if length, _ := lessSGRAt(line[index:]); length > 0 {
			index += length
			continue
		}
		out.WriteByte(line[index])
		index++
	}
	return out.String()
}

// lessSGR is style with one sequence's parameters applied, as a terminal applies them: 0 or none
// the default, 1 to 9 the attributes, 22 to 29 each off again, 30 to 37 and 90 to 97 the colours
// in front, 40 to 47 and 100 to 107 behind, 39 and 49 the default, and 38 and 48 a colour of 256
// by 5;N or one of RGB by 2;R;G;B. A colon is read as a semicolon.
func lessSGR(style tcell.Style, params string) tcell.Style {
	fields := strings.FieldsFunc(params, func(r rune) bool { return r == ';' || r == ':' })
	if len(fields) == 0 {
		return tcell.StyleDefault
	}
	number := func(index int) int {
		if index >= len(fields) {
			return -1
		}
		value, err := strconv.Atoi(fields[index])
		if err != nil {
			return -1
		}
		return value
	}
	for index := 0; index < len(fields); index++ {
		switch code := number(index); {
		case code == 0:
			style = tcell.StyleDefault
		case code == 1:
			style = style.Bold(true)
		case code == 2:
			style = style.Dim(true)
		case code == 3:
			style = style.Italic(true)
		case code == 4:
			style = style.Underline(true)
		case code == 5:
			style = style.Blink(true)
		case code == 7:
			style = style.Reverse(true)
		case code == 9:
			style = style.StrikeThrough(true)
		case code == 22:
			style = style.Bold(false).Dim(false)
		case code == 23:
			style = style.Italic(false)
		case code == 24:
			style = style.Underline(false)
		case code == 25:
			style = style.Blink(false)
		case code == 27:
			style = style.Reverse(false)
		case code == 29:
			style = style.StrikeThrough(false)
		case code >= 30 && code <= 37, code >= 90 && code <= 97:
			style = style.Foreground(tcell.PaletteColor(code%10 + (code/90)*8))
		case code >= 40 && code <= 47, code >= 100 && code <= 107:
			style = style.Background(tcell.PaletteColor(code%10 + (code/100)*8))
		case code == 39:
			style = style.Foreground(tcell.ColorDefault)
		case code == 49:
			style = style.Background(tcell.ColorDefault)
		case code == 38 || code == 48:
			var color tcell.Color
			switch number(index + 1) {
			case 5:
				palette := number(index + 2)
				if index += 2; palette < 0 || palette > 255 {
					continue
				}
				color = tcell.PaletteColor(palette)
			case 2:
				red, green, blue := number(index+2), number(index+3), number(index+4)
				if index += 4; min(red, green, blue) < 0 || max(red, green, blue) > 255 {
					continue
				}
				color = tcell.NewRGBColor(int32(red), int32(green), int32(blue))
			default:
				continue
			}
			if code == 38 {
				style = style.Foreground(color)
			} else {
				style = style.Background(color)
			}
		}
	}
	return style
}
