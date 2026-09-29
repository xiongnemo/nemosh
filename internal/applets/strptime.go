package applets

import (
	"strconv"
	"strings"
)

// strptime reads input against format as C's strptime does, into fields, leaving alone the ones
// the format does not name. It answers what is left of input, and false if the input does not
// fit. A blank in the format matches any run of blanks, none too, and a number may have blanks
// before it and fewer digits than the field allows, as glibc reads them.
//
// The conversions are glibc's, less the locale's: %Y %y %C %m %d %e %H %k %I %l %M %S %p %j, %b
// %B %h for a month's name and %a %A for a day's, which is read and set aside, %T %R %D %F %r,
// %z, %s, %n %t and %%.
func strptime(input, format string, fields *dateFields) (string, bool) {
	pm := -1
	for index := 0; index < len(format); index++ {
		if c := format[index]; c != '%' || index+1 == len(format) {
			if isBlank(c) {
				input = strings.TrimLeft(input, " \t\n\v\f\r")
				continue
			}
			if input == "" || input[0] != c {
				return input, false
			}
			input = input[1:]
			continue
		}
		index++
		var ok bool
		switch verb := format[index]; verb {
		case 'T', 'R', 'D', 'F', 'r':
			input, ok = strptime(input, map[byte]string{'T': "%H:%M:%S", 'R': "%H:%M", 'D': "%m/%d/%y", 'F': "%Y-%m-%d", 'r': "%I:%M:%S %p"}[verb], fields)
		case 'n', 't':
			input, ok = strings.TrimLeft(input, " \t\n\v\f\r"), true
		case '%':
			input, ok = strings.CutPrefix(input, "%")
		case 'b', 'B', 'h', 'a', 'A':
			input, ok = strptimeName(input, verb, fields)
		case 'p':
			input, pm, ok = strptimeMeridian(input)
		case 'z':
			input, ok = strptimeOffset(input, fields)
		case 's':
			var seconds int
			if input, seconds, ok = strptimeNumber(input, 19, -1<<62, 1<<62); ok {
				epoch := int64(seconds)
				fields.epoch = &epoch
			}
		default:
			input, ok = strptimeField(input, verb, fields)
		}
		if !ok {
			return input, false
		}
	}
	if pm >= 0 && fields.hour <= 12 {
		fields.hour = fields.hour%12 + pm*12
	}
	return input, true
}

// strptimeField is a numeric conversion: how many digits it takes, and the range they must be in.
func strptimeField(input string, verb byte, fields *dateFields) (string, bool) {
	spec, known := map[byte]struct {
		width, low, high int
		into             *int
	}{
		'Y': {4, 0, 9999, &fields.year}, 'm': {2, 1, 12, &fields.month}, 'd': {2, 1, 31, &fields.day},
		'e': {2, 1, 31, &fields.day}, 'H': {2, 0, 23, &fields.hour}, 'k': {2, 0, 23, &fields.hour},
		'I': {2, 1, 12, &fields.hour}, 'l': {2, 1, 12, &fields.hour}, 'M': {2, 0, 59, &fields.minute},
		'S': {2, 0, 61, &fields.second}, 'j': {3, 1, 366, nil}, 'y': {2, 0, 99, nil}, 'C': {2, 0, 99, nil},
	}[verb]
	if !known {
		return input, false
	}
	rest, value, ok := strptimeNumber(input, spec.width, spec.low, spec.high)
	if !ok {
		return input, false
	}
	switch {
	case spec.into != nil:
		*spec.into = value
	case verb == 'y' && value < 69:
		fields.year = 2000 + value
	case verb == 'y':
		fields.year = 1900 + value
	case verb == 'C':
		fields.year = value*100 + fields.year%100
	}
	return rest, true
}

// strptimeNumber reads up to width digits, after any blanks and an optional sign.
func strptimeNumber(input string, width, low, high int) (string, int, bool) {
	input = strings.TrimLeft(input, " \t")
	end := 0
	if end < len(input) && (input[end] == '-' || input[end] == '+') && low < 0 {
		end++
	}
	digits := end
	for end < len(input) && end-digits < width && input[end] >= '0' && input[end] <= '9' {
		end++
	}
	value, err := strconv.Atoi(input[:end])
	if end == digits || err != nil || value < low || value > high {
		return input, 0, false
	}
	return input[end:], value, true
}

var strptimeMonths = []string{"january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december"}
var strptimeDays = []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}

// strptimeName reads a month's name or a day's, whole or in three letters, in any case.
func strptimeName(input string, verb byte, fields *dateFields) (string, bool) {
	names := strptimeDays
	if verb == 'b' || verb == 'B' || verb == 'h' {
		names = strptimeMonths
	}
	lower := strings.ToLower(input)
	for index, name := range names {
		for _, spelling := range []string{name, name[:3]} {
			if strings.HasPrefix(lower, spelling) {
				if len(names) == 12 {
					fields.month = index + 1
				}
				return input[len(spelling):], true
			}
		}
	}
	return input, false
}

func strptimeMeridian(input string) (string, int, bool) {
	switch strings.ToUpper(input[:min(2, len(input))]) {
	case "AM":
		return input[2:], 0, true
	case "PM":
		return input[2:], 1, true
	}
	return input, -1, false
}

// strptimeOffset reads %z: Z, or a sign and hours with the minutes after them or a colon.
func strptimeOffset(input string, fields *dateFields) (string, bool) {
	input = strings.TrimLeft(input, " \t")
	if rest, ok := strings.CutPrefix(input, "Z"); ok {
		zero := 0
		fields.offset = &zero
		return rest, true
	}
	if input == "" || input[0] != '+' && input[0] != '-' {
		return input, false
	}
	digits := strings.Replace(input[1:min(len(input), 6)], ":", "", 1)
	end := 0
	for end < len(digits) && end < 4 && digits[end] >= '0' && digits[end] <= '9' {
		end++
	}
	if end != 2 && end != 4 {
		return input, false
	}
	hours, _ := strconv.Atoi(digits[:2])
	minutes := 0
	if end == 4 {
		minutes, _ = strconv.Atoi(digits[2:4])
	}
	offset := hours*3600 + minutes*60
	if input[0] == '-' {
		offset = -offset
	}
	fields.offset = &offset
	consumed := 1 + end
	if end == 4 && strings.Contains(input[1:min(len(input), 6)], ":") {
		consumed++
	}
	return input[consumed:], true
}

func isBlank(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r'
}
