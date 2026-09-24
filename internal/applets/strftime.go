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
		index++
		rendered, ok := strftimeConversion(when, format[index])
		if !ok {
			if strict {
				return "", fmt.Errorf("unsupported format: %%%c", format[index])
			}
			rendered = "%" + string(format[index])
		}
		out.WriteString(rendered)
	}
	return out.String(), nil
}

func strftimeConversion(when time.Time, verb byte) (string, bool) {
	if layout, ok := strftimeLayouts[verb]; ok {
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
	case '%':
		return "%", true
	}
	return "", false
}

// strftimeLayouts are the conversions Go's reference layout spells directly.
var strftimeLayouts = map[byte]string{
	'a': "Mon", 'A': "Monday", 'b': "Jan", 'h': "Jan", 'B': "January",
	'c': "Mon Jan _2 15:04:05 2006", 'd': "02", 'D': "01/02/06", 'e': "_2",
	'F': "2006-01-02", 'H': "15", 'I': "03", 'm': "01", 'M': "04", 'p': "PM",
	'r': "03:04:05 PM", 'R': "15:04", 'S': "05", 'T': "15:04:05", 'x': "01/02/06",
	'X': "15:04:05", 'y': "06", 'Y': "2006", 'z': "-0700", 'Z': "MST",
}
