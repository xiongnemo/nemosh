package applets

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// dateFields are a broken-down time, as C's struct tm holds one while strptime fills it in.
// offset is set by %z, which makes the time absolute rather than local.
type dateFields struct {
	year, month, day, hour, minute, second int
	offset                                 *int
	epoch                                  *int64
}

func fieldsOf(when time.Time) dateFields {
	return dateFields{year: when.Year(), month: int(when.Month()), day: when.Day(),
		hour: when.Hour(), minute: when.Minute(), second: when.Second()}
}

// in is the fields as a time in loc, normalised as mktime normalises them: February 30th is
// March 1st and 12:34:60 is 12:35.
func (f dateFields) in(loc *time.Location) time.Time {
	if f.epoch != nil {
		return time.Unix(*f.epoch, 0).In(loc)
	}
	if f.offset != nil {
		return time.Date(f.year, time.Month(f.month), f.day, f.hour, f.minute, f.second, 0, time.FixedZone("", *f.offset)).In(loc)
	}
	return time.Date(f.year, time.Month(f.month), f.day, f.hour, f.minute, f.second, 0, loc)
}

// dateStringFormats are the ones busybox's parse_datestr tries in turn (libbb/time.c:23-44).
var dateStringFormats = []string{
	"%R", "%T", "%m.%d-%R", "%m.%d-%T", "%Y.%m.%d-%R", "%Y.%m.%d-%T", "%b %d %T %Y",
	"%Y-%m-%d %R", "%Y-%m-%d %T", "%Y-%m-%d %R %z", "%Y-%m-%d %T %z", "%Y-%m-%d %H", "%Y-%m-%d",
}

// parseDateString is libbb's parse_datestr and validate_tm_time: TIME as date -d and touch -d
// and -t read it, the fields it does not name taken from base. It is one of the formats above,
// `@SECONDS`, or `[[[[[YY]YY]MM]DD]hh]mm[.ss]`, which is touch -t's.
func parseDateString(input string, base time.Time) (time.Time, error) {
	invalid := fmt.Errorf("invalid date '%s'", input)
	for _, format := range dateStringFormats {
		fields := fieldsOf(base)
		if rest, ok := strptime(input, format, &fields); ok && rest == "" {
			return fields.in(base.Location()), nil
		}
	}
	if strings.HasPrefix(input, "@") {
		seconds, err := strconv.ParseInt(input[1:], 10, 64)
		if err != nil {
			return time.Time{}, invalid
		}
		return time.Unix(seconds, 0).In(base.Location()), nil
	}
	fields, ok := touchStampFields(input, base)
	if !ok {
		return time.Time{}, invalid
	}
	return fields.in(base.Location()), nil
}

// touchStampFields reads `[[[[[YY]YY]MM]DD]hh]mm[.ss]`, the seconds 0 unless given. A two-digit
// year is put within fifty years of base's, as busybox puts it.
func touchStampFields(input string, base time.Time) (dateFields, bool) {
	stamp, seconds, dotted := strings.Cut(input, ".")
	if !allDigits(stamp) || dotted && !allDigits(seconds) {
		return dateFields{}, false
	}
	fields := fieldsOf(base)
	fields.second = 0
	pairs := make([]int, 0, 6)
	for index := 0; index+2 <= len(stamp); index += 2 {
		value, _ := strconv.Atoi(stamp[index : index+2])
		pairs = append(pairs, value)
	}
	switch len(stamp) {
	case 2:
		fields.minute = pairs[0]
	case 4:
		fields.hour, fields.minute = pairs[0], pairs[1]
	case 6:
		fields.day, fields.hour, fields.minute = pairs[0], pairs[1], pairs[2]
	case 8:
		fields.month, fields.day, fields.hour, fields.minute = pairs[0], pairs[1], pairs[2], pairs[3]
	case 10:
		current := base.Year()
		year := pairs[0] + current/100*100
		if year < current-50 {
			year += 100
		} else if year > current+50 {
			year -= 100
		}
		fields.year, fields.month, fields.day, fields.hour, fields.minute = year, pairs[1], pairs[2], pairs[3], pairs[4]
	case 12:
		fields.year, fields.month, fields.day = pairs[0]*100+pairs[1], pairs[2], pairs[3]
		fields.hour, fields.minute = pairs[4], pairs[5]
	default:
		return dateFields{}, false
	}
	if dotted {
		fields.second, _ = strconv.Atoi(seconds)
	}
	ok := fields.second <= 60 && fields.minute <= 59 && fields.hour <= 23 && fields.day <= 31 && fields.month <= 12
	return fields, ok
}

func allDigits(text string) bool {
	for index := 0; index < len(text); index++ {
		if text[index] < '0' || text[index] > '9' {
			return false
		}
	}
	return text != ""
}
