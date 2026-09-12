package applets

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// cal, the calendar, following busybox-w32's util-linux/cal.c closely enough that the bytes
// match. Two of the things it does look like faults and are not:
//
//   - **September 1752 is short.** Britain and its colonies dropped eleven days moving to
//     the Gregorian calendar, so `cal 9 1752` runs 1, 2, 14, 15. Every `cal` since the
//     original BSD one prints it that way, and a version that "corrected" it would be wrong
//     about the weekday of every date before it.
//   - **A week row that is entirely blank prints as spaces**, not as an empty line. The
//     reference trims trailing spaces by walking back from the end until it meets a
//     non-space -- and on an all-space row it never meets one, so nothing is trimmed
//     (cal.c:310-322). A month ending in a blank week therefore ends with a line of 21
//     spaces. Reproduced rather than tidied, because output that differs by whitespace is
//     still output that differs.
//
// `-j`, busybox's day-of-year form, is **refused by name**. Its own source carries an audit
// note saying `cal -j 1752` is wrong (cal.c:25), and reproducing a layout the reference
// admits is broken is worth less than saying the option is not here.

const (
	// calWeekLength is seven three-character columns, less the space after the last.
	calWeekLength = 20
	calHeadSep    = 2
	// calRowLength is what a row is *built* in: seven columns of three, trailing space
	// and all. The difference from calWeekLength is where the 21-space blank row comes
	// from.
	calRowLength = 21
	// calFirstMissingDay is 3 September 1752, counted from 1 January 1, and calMissingDays
	// is how many vanished there.
	calFirstMissingDay = 639787
	calMissingDays     = 11
	calBlank           = -1
)

var (
	calDaysInMonth = [13]int{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	// calSeptember1752 is the month as it was actually lived.
	calSeptember1752 = []int{1, 2, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30}
	calMonthNames    = [12]string{"January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December"}
	calDayNames = [7]string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}
)

func newCalApplet() Applet {
	return simpleApplet{name: "cal", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(args, "my", "")
		if err != nil {
			return err
		}
		month, year, err := calRequest(operands, options.has('y'))
		if err != nil {
			return err
		}
		weekstart := 0
		if options.has('m') {
			weekstart = 1
		}
		if month == 0 {
			return writeCalendarYear(stdout, year, weekstart)
		}
		return writeCalendarMonth(stdout, month, year, weekstart)
	}}
}

// calRequest reads the operands: none, a year, or a month and a year.
//
// A month of 0 means the whole year, which is how `cal 2026` and `cal -y` arrive here.
func calRequest(operands []string, wholeYear bool) (int, int, error) {
	now := time.Now()
	switch len(operands) {
	case 0:
		if wholeYear {
			return 0, now.Year(), nil
		}
		return int(now.Month()), now.Year(), nil
	case 1:
		year, err := calNumber(operands[0], 1, 9999)
		if err != nil {
			return 0, 0, err
		}
		return 0, year, nil
	case 2:
		month, err := calNumber(operands[0], 1, 12)
		if err != nil {
			return 0, 0, err
		}
		year, err := calNumber(operands[1], 1, 9999)
		if err != nil {
			return 0, 0, err
		}
		if wholeYear {
			return 0, year, nil
		}
		return month, year, nil
	}
	return 0, 0, fmt.Errorf("extra operand '%s'", operands[2])
}

func calNumber(text string, low, high int) (int, error) {
	value, err := strconv.Atoi(text)
	if err != nil || value < low || value > high {
		return 0, fmt.Errorf("invalid number '%s'", text)
	}
	return value, nil
}

func writeCalendarMonth(out io.Writer, month, year, weekstart int) error {
	days := calDayArray(month, year, weekstart)
	title := fmt.Sprintf("%s %d", calMonthNames[month-1], year)
	// Left-padded rather than centred in a fixed field: the reference prints the pad and
	// then the title, so there is no trailing whitespace on this line.
	lines := []string{
		strings.Repeat(" ", (calWeekLength-len(title))/2) + title,
		calHeadings(weekstart),
	}
	for row := 0; row < 6; row++ {
		lines = append(lines, calTrim(calBuildRow(days[row*7:row*7+7])))
	}
	_, err := io.WriteString(out, strings.Join(lines, "\n")+"\n")
	return err
}

func writeCalendarYear(out io.Writer, year, weekstart int) error {
	var page strings.Builder
	// The year, centred over three month blocks and the two gaps between them, then a
	// blank line -- the reference's `puts("\n")`, which writes one newline and adds
	// another.
	page.WriteString(calCenter(strconv.Itoa(year), calWeekLength*3+calHeadSep*2, 0) + "\n\n")
	months := make([][]int, 12)
	for index := range months {
		months[index] = calDayArray(index+1, year, weekstart)
	}
	headings := calHeadings(weekstart)
	separator := strings.Repeat(" ", calHeadSep)
	for month := 0; month < 12; month += 3 {
		page.WriteString(calCenter(calMonthNames[month], calWeekLength, calHeadSep))
		page.WriteString(calCenter(calMonthNames[month+1], calWeekLength, calHeadSep))
		page.WriteString(calCenter(calMonthNames[month+2], calWeekLength, 0))
		page.WriteString("\n" + headings + separator + headings + separator + headings + "\n")
		for row := 0; row < 6; row++ {
			page.WriteString(calTrim(calYearRow(months[month:month+3], row)) + "\n")
		}
	}
	_, err := io.WriteString(out, page.String())
	return err
}

// calYearRow lays three months side by side.
//
// The line is 79 characters wide before trimming, because the reference blanks its whole
// buffer once and writes the three blocks into it -- which is why an all-blank row in the
// year view is 79 spaces where the same row in the month view is 21.
func calYearRow(months [][]int, row int) string {
	line := []byte(strings.Repeat(" ", 79))
	for which, days := range months {
		copy(line[which*(calWeekLength+2):], calBuildRow(days[row*7:row*7+7]))
	}
	return string(line)
}

// calBuildRow renders one week as seven three-character columns.
func calBuildRow(week []int) string {
	var row strings.Builder
	for _, day := range week {
		if day == calBlank {
			row.WriteString("   ")
			continue
		}
		fmt.Fprintf(&row, "%2d ", day)
	}
	return row.String()
}

// calTrim removes trailing spaces -- unless there is nothing else, which is the rule that
// leaves a blank week as a line of spaces.
func calTrim(line string) string {
	trimmed := strings.TrimRight(line, " ")
	if trimmed == "" {
		return line
	}
	return trimmed
}

// calCenter centres text in a field and adds a separator after it, as the reference does.
func calCenter(text string, width, separate int) string {
	remaining := width - len(text)
	return strings.Repeat(" ", remaining/2) + text +
		strings.Repeat(" ", remaining/2+remaining%2+separate)
}

func calHeadings(weekstart int) string {
	names := make([]string, 0, 7)
	for index := 0; index < 7; index++ {
		names = append(names, calDayNames[(index+weekstart)%7])
	}
	return strings.Join(names, " ")
}

// calDayArray places each day of a month in its column, with calBlank for the gaps.
func calDayArray(month, year, weekstart int) []int {
	days := make([]int, 42)
	for index := range days {
		days[index] = calBlank
	}
	if month == 9 && year == 1752 {
		// The eleven days that did not happen. Laid out from a fixed table because the
		// arithmetic below cannot express a month with a hole in it.
		for offset, day := range calSeptember1752 {
			days[offset+2-weekstart] = day
		}
		return days
	}
	weekday := calWeekdayOfFirst(month, year, weekstart)
	count := calDaysInMonth[month]
	if month == 2 && calLeapYear(year) {
		count++
	}
	for day := 1; day <= count; day++ {
		days[weekday] = day
		weekday++
	}
	return days
}

// calWeekdayOfFirst answers which column the first of the month falls in.
//
// 1 January of year 1 was a Saturday, and the eleven days lost in 1752 are subtracted once
// the count passes them -- which is what keeps every date before the reformation on the
// weekday it was actually called.
func calWeekdayOfFirst(month, year, weekstart int) int {
	dayOfYear := 1
	if month > 2 && calLeapYear(year) {
		dayOfYear++
	}
	for index := month - 1; index > 0; index-- {
		dayOfYear += calDaysInMonth[index]
	}
	const saturday = 6
	total := (year-1)*365 + calLeapYearsBefore(year-1) + dayOfYear
	weekday := (total - 1 + saturday) % 7
	if total >= calFirstMissingDay {
		weekday = (total - 1 + saturday - calMissingDays) % 7
	}
	return ((weekday-weekstart)%7 + 7) % 7
}

// calLeapYear is the rule in force at the time: every fourth year up to 1752, and the
// Gregorian rule after it.
func calLeapYear(year int) bool {
	if year <= 1752 {
		return year%4 == 0
	}
	return (year%4 == 0 && year%100 != 0) || year%400 == 0
}

func calLeapYearsBefore(year int) int {
	centuries, quadCenturies := 0, 0
	if year > 1700 {
		centuries = year/100 - 17
	}
	if year > 1600 {
		quadCenturies = (year - 1600) / 400
	}
	return year/4 - centuries + quadCenturies
}
