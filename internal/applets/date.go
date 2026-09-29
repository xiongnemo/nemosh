package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const busyBoxDateDefaultFormat = "%a %b %e %H:%M:%S %Z %Y"

// date is busybox's (coreutils/date.c): `date [-uR] [-I[SPEC]] [-d TIME [-D FMT]] [-r FILE]
// [+FMT]`. -d prints TIME rather than now, read as parse_datestr reads it (datestr.go) or, with
// -D, by that strptime format. -r prints FILE's modification time, -R the RFC 2822 form, and -I
// the ISO 8601 one to the precision SPEC names: date, hours, minutes, seconds or ns. -u works in
// UTC. FMT is strftime's, with busybox's %N for the nanoseconds and %3N for the first three.
//
// -d took `@SECONDS` alone, and -r, -R, -I, -D and the long options were refused, so
// `date -d 2020-01-02 +%s`, `date -r build.log` and `date -I` all failed.
//
// Setting the clock, with -s or a TIME operand, is refused as busybox refuses it without the
// privilege: `can't set date: Operation not permitted`.
type dateApplet struct {
	now func() time.Time
}

func newDateApplet() Applet {
	return dateApplet{now: time.Now}
}

func (dateApplet) Name() string {
	return "date"
}

func (a dateApplet) Run(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	request, err := parseDateArgs(args, optionsPermute(ProcessViewFromContext(ctx)))
	if err != nil {
		return writeDateDiagnostic(stderr, err.Error())
	}
	stamp, err := a.dateTime(ctx, request)
	if err != nil {
		return writeDateDiagnostic(stderr, err.Error())
	}
	formatted, err := formatDate(stamp, request.outputFormat(stamp))
	if err != nil {
		return writeDateDiagnostic(stderr, err.Error())
	}
	_, err = fmt.Fprintln(stdout, formatted)
	return err
}

// dateTime is the time to print: now or FILE's, or TIME read against it at midnight.
func (a dateApplet) dateTime(ctx context.Context, request dateRequest) (time.Time, error) {
	stamp := a.now()
	if request.reference != "" {
		native, err := resolveHostPath(ProcessViewFromContext(ctx), request.reference)
		if err != nil {
			return time.Time{}, fmt.Errorf("date: %v", err)
		}
		info, err := os.Stat(native)
		if err != nil {
			return time.Time{}, fmt.Errorf("date: %v", cannotStat(request.reference, err))
		}
		stamp = info.ModTime()
	}
	if request.utc {
		stamp = stamp.UTC()
	}
	if !request.hasInput {
		return stamp, nil
	}
	midnight := time.Date(stamp.Year(), stamp.Month(), stamp.Day(), 0, 0, 0, 0, stamp.Location())
	parsed, err := parseDateString(request.input, midnight)
	if request.inputFormat != "" {
		fields := fieldsOf(midnight)
		_, ok := strptime(request.input, request.inputFormat, &fields)
		parsed, err = fields.in(midnight.Location()), nil
		if !ok {
			err = fmt.Errorf("invalid date '%s'", request.input)
		}
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("date: %v", err)
	}
	if request.setting {
		return time.Time{}, fmt.Errorf("date: cannot set date: Operation not permitted")
	}
	return parsed, nil
}

type dateRequest struct {
	utc, rfc2822, setting, hasInput, hasFormat bool
	input, inputFormat, reference, format      string
	// iso is -I's precision: -1 without -I, then date, hours, minutes, seconds and ns.
	iso int
}

// outputFormat is FMT, or the form -I or -R asks for, or busybox's default.
func (r dateRequest) outputFormat(stamp time.Time) string {
	switch {
	case r.hasFormat:
		return expandNanoseconds(r.format, stamp)
	case r.iso == 0:
		return "%Y-%m-%d"
	case r.iso > 0:
		layout := []string{"", "%Y-%m-%dT%H", "%Y-%m-%dT%H:%M", "%Y-%m-%dT%H:%M:%S", "%Y-%m-%dT%H:%M:%S"}[r.iso]
		if r.iso == 4 {
			layout += fmt.Sprintf(",%09d", stamp.Nanosecond())
		}
		return layout + stamp.Format("-07:00")
	case r.rfc2822:
		return "%a, %d %b %Y %H:%M:%S %z"
	}
	return busyBoxDateDefaultFormat
}

// expandNanoseconds is busybox's %N and %[n]N: the nanoseconds, or their first n digits.
func expandNanoseconds(format string, stamp time.Time) string {
	var out strings.Builder
	for index := 0; index < len(format); index++ {
		if format[index] != '%' || index+1 >= len(format) {
			out.WriteByte(format[index])
			continue
		}
		end := index + 1
		for end < len(format) && format[end] >= '0' && format[end] <= '9' {
			end++
		}
		if end >= len(format) || format[end] != 'N' {
			out.WriteString(format[index : index+2])
			index++
			continue
		}
		digits, width := fmt.Sprintf("%09d", stamp.Nanosecond()), 9
		if given, err := strconv.Atoi(format[index+1 : end]); err == nil && given > 0 {
			width = given
		}
		if width < 9 {
			digits = digits[:width]
		}
		out.WriteString(strings.Repeat("0", max(0, width-9)) + digits)
		index = end
	}
	return out.String()
}

// formatDate is `date +FORMAT`, through the one strftime (strftime.go), strictly: a
// conversion this build does not know is refused rather than printed as written.
func formatDate(stamp time.Time, format string) (string, error) {
	formatted, err := strftime(stamp, format, true)
	if err != nil {
		return "", fmt.Errorf("date: %v", err)
	}
	return formatted, nil
}

func writeDateDiagnostic(stderr io.Writer, message string) error {
	if _, err := fmt.Fprintln(stderr, message); err != nil {
		return err
	}
	return ErrExitFalse
}
