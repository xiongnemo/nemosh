package applets

import (
	"strconv"
	"strings"
	"time"
)

// GNU's relative items, which date -d and touch -d read after busybox's formats have all failed,
// so that what busybox reads is read as before: `date -d yesterday`, `date -d '2 days ago'`,
// `touch -d 'last monday'`, `date -d 'tomorrow 09:00'`. busybox's date says `invalid date` to
// each; Debian's is GNU's, and the rules followed are its parse_datetime's (parse-datetime.y):
//
//   - now and today are now, and yesterday and tomorrow a day either side at the same time.
//   - N UNIT, +N UNIT and -N UNIT move by N, and ago after an item turns that item round; next
//     and last are 1 and -1, and this 0. Years, months, weeks, fortnights and days are of the
//     calendar, so a month ago from 31 March is 3 March, as mktime has it; hours, minutes and
//     seconds are durations.
//   - A day of the week is the next such day, or today, at midnight; next is the one after that
//     when today is that day, and last the one before.
//   - What is not an item is the date or time of day, read as busybox reads it, and the items
//     move it. A day of the week then counts only when that was a time of day.

// relativeDate is what the items ask for.
type relativeDate struct {
	years, months, days int
	clock               time.Duration
	weekday, ordinal    int
	items               int
}

// parseDateInput is TIME as date -d and touch -d read it: busybox's formats against base, then
// GNU's relative items, moving now, or what is left of TIME read against base.
func parseDateInput(input string, base, now time.Time) (time.Time, error) {
	when, err := parseDateString(input, base)
	if err == nil {
		return when, nil
	}
	if moved, ok := parseRelativeDate(input, base, now); ok {
		return moved, nil
	}
	return time.Time{}, err
}

func parseRelativeDate(input string, base, now time.Time) (time.Time, bool) {
	words := strings.Fields(input)
	relative := relativeDate{weekday: -1}
	var rest []string
	for index := 0; index < len(words); index++ {
		used, ok := relative.item(words, index)
		switch {
		case !ok:
			return time.Time{}, false
		case used == 0:
			rest = append(rest, words[index])
		default:
			index += used - 1
		}
	}
	if relative.items == 0 {
		return time.Time{}, false
	}
	when, timeOnly := now, false
	if len(rest) > 0 {
		text := strings.Join(rest, " ")
		parsed, err := parseDateString(text, base)
		if err != nil {
			return time.Time{}, false
		}
		when, timeOnly = parsed, isTimeOfDay(text, base)
	}
	if relative.weekday >= 0 && (len(rest) == 0 || timeOnly) {
		when = relative.onWeekday(when, timeOnly)
	}
	return when.AddDate(relative.years, relative.months, relative.days).Add(relative.clock), true
}

// item reads the item at words[index], answering how many words it took: 0 for a word that is
// no item, and false for one that begins an item and does not end it.
func (r *relativeDate) item(words []string, index int) (int, bool) {
	word := strings.ToLower(words[index])
	switch word {
	case "now", "today":
		r.items++
		return 1, true
	case "yesterday":
		r.items++
		r.days--
		return 1, true
	case "tomorrow":
		r.items++
		r.days++
		return 1, true
	case "ago":
		return 0, false
	}
	if day, ok := weekdayNamed(word); ok {
		return 1, r.setWeekday(day, 0)
	}
	if ordinal, ok := dateOrdinal(word); ok {
		if index+1 >= len(words) {
			return 0, false
		}
		if day, ok := weekdayNamed(strings.ToLower(words[index+1])); ok {
			return 2, r.setWeekday(day, ordinal)
		}
		return r.unit(words, index+1, ordinal, 1)
	}
	if count, err := strconv.Atoi(word); err == nil {
		return r.unit(words, index+1, count, 1)
	}
	if _, ok := dateUnit(word); ok {
		return r.unit(words, index, 1, 0)
	}
	return 0, true
}

// unit reads the unit at words[index], moved count of them, and an ago after it. taken is how many
// words were read before the unit.
func (r *relativeDate) unit(words []string, index, count, taken int) (int, bool) {
	if index >= len(words) {
		return 0, false
	}
	add, ok := dateUnit(strings.ToLower(words[index]))
	if !ok {
		return 0, false
	}
	used := taken + 1
	if index+1 < len(words) && strings.EqualFold(words[index+1], "ago") {
		count, used = -count, used+1
	}
	add(r, count)
	r.items++
	return used, true
}

func (r *relativeDate) setWeekday(day, ordinal int) bool {
	if r.weekday >= 0 {
		return false
	}
	r.weekday, r.ordinal = day, ordinal
	r.items++
	return true
}

// onWeekday is GNU's: tm_mday += (day - wday + 7) % 7 + 7 * (ordinal - (0 < ordinal && wday != day)),
// at midnight unless a time of day was given.
func (r relativeDate) onWeekday(when time.Time, keepTime bool) time.Time {
	current := int(when.Weekday())
	shift := (r.weekday - current + 7) % 7
	weeks := r.ordinal
	if r.ordinal > 0 && current != r.weekday {
		weeks--
	}
	hour, minute, second := 0, 0, 0
	if keepTime {
		hour, minute, second = when.Clock()
	}
	return time.Date(when.Year(), when.Month(), when.Day()+shift+7*weeks, hour, minute, second, 0, when.Location())
}

// dateUnit is what moving by one of a unit adds, by its singular, plural or short name.
func dateUnit(word string) (func(*relativeDate, int), bool) {
	switch strings.TrimSuffix(word, "s") {
	case "year":
		return func(r *relativeDate, n int) { r.years += n }, true
	case "month":
		return func(r *relativeDate, n int) { r.months += n }, true
	case "fortnight":
		return func(r *relativeDate, n int) { r.days += 14 * n }, true
	case "week":
		return func(r *relativeDate, n int) { r.days += 7 * n }, true
	case "day":
		return func(r *relativeDate, n int) { r.days += n }, true
	case "hour":
		return func(r *relativeDate, n int) { r.clock += time.Duration(n) * time.Hour }, true
	case "minute", "min":
		return func(r *relativeDate, n int) { r.clock += time.Duration(n) * time.Minute }, true
	case "second", "sec":
		return func(r *relativeDate, n int) { r.clock += time.Duration(n) * time.Second }, true
	}
	return nil, false
}

// dateOrdinal is next, last and this, as GNU's tORDINAL has them.
func dateOrdinal(word string) (int, bool) {
	switch word {
	case "next":
		return 1, true
	case "last":
		return -1, true
	case "this":
		return 0, true
	}
	return 0, false
}

// weekdayNamed is a day of the week by its name or the abbreviations GNU takes, 0 for Sunday.
func weekdayNamed(word string) (int, bool) {
	word = strings.TrimSuffix(word, ",")
	for day, names := range [...][]string{{"sunday", "sun"}, {"monday", "mon"}, {"tuesday", "tue", "tues"},
		{"wednesday", "wed", "wednes"}, {"thursday", "thu", "thur", "thurs"}, {"friday", "fri"}, {"saturday", "sat"}} {
		for _, name := range names {
			if word == name {
				return day, true
			}
		}
	}
	return 0, false
}

// isTimeOfDay is whether text is a time of day alone, busybox's %R or %T.
func isTimeOfDay(text string, base time.Time) bool {
	for _, format := range []string{"%R", "%T"} {
		fields := fieldsOf(base)
		if rest, ok := strptime(text, format, &fields); ok && rest == "" {
			return true
		}
	}
	return false
}
