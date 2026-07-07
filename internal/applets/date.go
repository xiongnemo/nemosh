package applets

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const busyBoxDateDefaultFormat = "%a %b %e %H:%M:%S %Z %Y"

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
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	request, err := parseDateArgs(args)
	if err != nil {
		return writeDateDiagnostic(stderr, err.Error())
	}
	stamp := a.dateTime(request)
	formatted, err := formatDate(stamp, request.format)
	if err != nil {
		return writeDateDiagnostic(stderr, err.Error())
	}
	_, err = fmt.Fprintln(stdout, formatted)
	return err
}

func (a dateApplet) dateTime(request dateRequest) time.Time {
	stamp := a.now()
	if request.hasEpoch {
		stamp = time.Unix(request.epoch, 0)
	}
	if request.utc {
		stamp = stamp.UTC()
	}
	return stamp
}

type dateRequest struct {
	utc      bool
	hasEpoch bool
	epoch    int64
	format   string
}

func parseDateArgs(args []string) (dateRequest, error) {
	request := dateRequest{format: busyBoxDateDefaultFormat}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "-u":
			request.utc = true
		case arg == "-d":
			if index+1 >= len(args) {
				return dateRequest{}, fmt.Errorf("date: missing operand for -d")
			}
			epoch, err := parseDateEpoch(args[index+1])
			if err != nil {
				return dateRequest{}, err
			}
			request.hasEpoch = true
			request.epoch = epoch
			index++
		case arg == "-s" || arg == "-r" || arg == "-R" || arg == "-I":
			return dateRequest{}, fmt.Errorf("date: unsupported option: %s", arg)
		case strings.HasPrefix(arg, "--"):
			return dateRequest{}, fmt.Errorf("date: unsupported option: %s", arg)
		case strings.HasPrefix(arg, "+"):
			request.format = strings.TrimPrefix(arg, "+")
		case strings.HasPrefix(arg, "-"):
			return dateRequest{}, fmt.Errorf("date: unsupported option: %s", arg)
		default:
			return dateRequest{}, fmt.Errorf("date: unsupported date: %s", arg)
		}
	}
	return request, nil
}

func parseDateEpoch(input string) (int64, error) {
	if !strings.HasPrefix(input, "@") || len(input) == 1 {
		return 0, fmt.Errorf("date: unsupported date: %s", input)
	}
	epoch, err := strconv.ParseInt(input[1:], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("date: invalid date: %s", input)
	}
	return epoch, nil
}

func formatDate(stamp time.Time, format string) (string, error) {
	var builder strings.Builder
	for index := 0; index < len(format); index++ {
		char := format[index]
		if char != '%' {
			builder.WriteByte(char)
			continue
		}
		if index+1 >= len(format) {
			return "", fmt.Errorf("date: unsupported format: %%")
		}
		index++
		part, err := formatDateToken(stamp, format[index])
		if err != nil {
			return "", err
		}
		builder.WriteString(part)
	}
	return builder.String(), nil
}

func formatDateToken(stamp time.Time, token byte) (string, error) {
	switch token {
	case 'Y':
		return stamp.Format("2006"), nil
	case 'm':
		return stamp.Format("01"), nil
	case 'd':
		return stamp.Format("02"), nil
	case 'H':
		return stamp.Format("15"), nil
	case 'M':
		return stamp.Format("04"), nil
	case 'S':
		return stamp.Format("05"), nil
	case 'Z':
		return stamp.Format("MST"), nil
	case 'a':
		return stamp.Format("Mon"), nil
	case 'b':
		return stamp.Format("Jan"), nil
	case 'e':
		return stamp.Format("_2"), nil
	case 's':
		return strconv.FormatInt(stamp.Unix(), 10), nil
	case '%':
		return "%", nil
	default:
		return "", fmt.Errorf("date: unsupported format: %%%c", token)
	}
}

func writeDateDiagnostic(stderr io.Writer, message string) error {
	if _, err := fmt.Fprintln(stderr, message); err != nil {
		return err
	}
	return ErrExitFalse
}
