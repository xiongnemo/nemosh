package applets

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// strftime renders C's strftime conversions, in the C locale.
//
// **One implementation.** There were two -- `date` knew a dozen conversions and `ts` fewer,
// each its own list -- and printf's `%(fmt)T` would have made a third. It is the set glibc
// and busybox give: the names and numbers of the date, the time in 12 and 24 hours, the day
// and week of the year, the ISO week-based year, the offset and the zone.
//
// strict decides an unknown conversion: an error for `date`, which refuses a format it
// cannot honour, and the conversion as written for `ts`, whose formats are stamps and where
// a typo is better shown than swallowed.
func strftime(when time.Time, format string, strict bool) (string, error) {
	var out strings.Builder
	for index := 0; index < len(format); index++ {
		if format[index] != '%' {
			out.WriteByte(format[index])
			continue
		}
		if index+1 >= len(format) {
			if strict {
				return "", fmt.Errorf("unsupported format: %%")
			}
			out.WriteByte('%')
			continue
		}
		start := index
		flags, width, used := strftimeModifiers(format[index+1:])
		index += 1 + used
		if index >= len(format) {
			if strict {
				return "", fmt.Errorf("unsupported format: %s", format[start:])
			}
			out.WriteString(format[start:])
			continue
		}
		rendered, ok := strftimeConversion(when, format[index])
		if !ok {
			if strict {
				return "", fmt.Errorf("unsupported format: %s", format[start:index+1])
			}
			out.WriteString(format[start : index+1])
			continue
		}
		out.WriteString(strftimePad(rendered, format[index], flags, width))
	}
	return out.String(), nil
}

// Strftime is strftime for the shell's prompt, whose \D{format} is one: an unknown conversion
// is left as written, as ts leaves it.
func Strftime(when time.Time, format string) string {
	return strftimeLike(when, format)
}

func strftimeConversion(when time.Time, verb byte) (string, bool) {
	if layout, ok := strftimeLayouts(verb); ok {
		return when.Format(layout), true
	}
	hour12 := when.Hour() % 12
	if hour12 == 0 {
		hour12 = 12
	}
	isoYear, isoWeek := when.ISOWeek()
	switch verb {
	case 'C':
		return fmt.Sprintf("%02d", when.Year()/100), true
	case 'g':
		return fmt.Sprintf("%02d", isoYear%100), true
	case 'G':
		return strconv.Itoa(isoYear), true
	case 'j':
		return fmt.Sprintf("%03d", when.YearDay()), true
	case 'k':
		return fmt.Sprintf("%2d", when.Hour()), true
	case 'l':
		return fmt.Sprintf("%2d", hour12), true
	case 'n':
		return "\n", true
	case 't':
		return "\t", true
	case 'P':
		return strings.ToLower(when.Format("PM")), true
	case 's':
		return strconv.FormatInt(when.Unix(), 10), true
	case 'u':
		return strconv.Itoa((int(when.Weekday())+6)%7 + 1), true
	case 'w':
		return strconv.Itoa(int(when.Weekday())), true
	case 'U':
		return fmt.Sprintf("%02d", (when.YearDay()+6-int(when.Weekday()))/7), true
	case 'W':
		return fmt.Sprintf("%02d", (when.YearDay()+6-(int(when.Weekday())+6)%7)/7), true
	case 'V':
		return fmt.Sprintf("%02d", isoWeek), true
	case 'q':
		// The quarter, GNU's.
		return strconv.Itoa((int(when.Month())-1)/3 + 1), true
	case '%':
		return "%", true
	}
	return "", false
}

// strftimeLayouts are the conversions Go's reference layout spells directly.
func strftimeLayouts(key byte) (string, bool) {
	switch key {
	case 'a':
		return "Mon", true
	case 'A':
		return "Monday", true
	case 'b', 'h':
		return "Jan", true
	case 'B':
		return "January", true
	case 'c':
		return "Mon Jan _2 15:04:05 2006", true
	case 'd':
		return "02", true
	case 'D', 'x':
		return "01/02/06", true
	case 'e':
		return "_2", true
	case 'F':
		return "2006-01-02", true
	case 'H':
		return "15", true
	case 'I':
		return "03", true
	case 'm':
		return "01", true
	case 'M':
		return "04", true
	case 'p':
		return "PM", true
	case 'r':
		return "03:04:05 PM", true
	case 'R':
		return "15:04", true
	case 'S':
		return "05", true
	case 'T', 'X':
		return "15:04:05", true
	case 'y':
		return "06", true
	case 'Y':
		return "2006", true
	case 'z':
		return "-0700", true
	case 'Z':
		return "MST", true
	}
	var zero string
	return zero, false
}
