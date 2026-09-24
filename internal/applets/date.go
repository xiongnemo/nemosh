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
