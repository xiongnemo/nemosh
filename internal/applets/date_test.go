package applets

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDateApplet_returnsName_whenConstructed(t *testing.T) {
	// Given
	applet := newDateApplet()

	// When
	got := applet.Name()

	// Then
	if got != "date" {
		t.Fatalf("expected date applet name, got %q", got)
	}
}

func TestDateApplet_printsFormattedUTCEpoch_whenRunWithDateAndFormat(t *testing.T) {
	// Given
	applet := newDateApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-u", "-d", "@0", "+%Y-%m-%dT%H:%M:%SZ"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected date to succeed, got %v", err)
	}
	if got, want := stdout.String(), "1970-01-01T00:00:00Z\n"; got != want {
		t.Fatalf("expected UTC epoch output %q, got %q", want, got)
	}
}

func TestDateApplet_printsUnixSeconds_whenRunWithEpochAndSecondsFormat(t *testing.T) {
	// Given
	applet := newDateApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-u", "-d", "@1", "+%s"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected date to succeed, got %v", err)
	}
	if got, want := stdout.String(), "1\n"; got != want {
		t.Fatalf("expected Unix seconds output %q, got %q", want, got)
	}
}

func TestDateApplet_printsBusyBoxDefaultFormat_whenRunWithUTCEpoch(t *testing.T) {
	// Given
	applet := newDateApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-u", "-d", "@0"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected date to succeed, got %v", err)
	}
	if got, want := stdout.String(), "Thu Jan  1 00:00:00 UTC 1970\n"; got != want {
		t.Fatalf("expected BusyBox default output %q, got %q", want, got)
	}
}

func TestDateApplet_printsLiteralFormat_whenRunWithLiteralFormat(t *testing.T) {
	// Given
	applet := newDateApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-u", "-d", "@0", "+literal"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected date to succeed, got %v", err)
	}
	if got, want := stdout.String(), "literal\n"; got != want {
		t.Fatalf("expected literal format output %q, got %q", want, got)
	}
}

func TestDateApplet_printsInjectedNow_whenRunWithoutOperands(t *testing.T) {
	// Given
	applet := dateApplet{now: func() time.Time {
		return time.Date(2026, time.July, 8, 9, 10, 11, 0, time.FixedZone("TST", 3600))
	}}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected date to succeed, got %v", err)
	}
	if got, want := stdout.String(), "Wed Jul  8 09:10:11 TST 2026\n"; got != want {
		t.Fatalf("expected injected-now default output %q, got %q", want, got)
	}
}

func TestDateApplet_returnsErrExitFalseAndDiagnostic_whenRunWithUnsupportedFeature(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "set time", args: []string{"-s", "@0"}},
		{name: "reference file", args: []string{"-r", "file.txt"}},
		{name: "rfc 2822", args: []string{"-R"}},
		{name: "iso 8601", args: []string{"-I"}},
		{name: "long option", args: []string{"--utc"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			applet := newDateApplet()
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			// When
			err := applet.Run(context.Background(), tt.args, &bytes.Buffer{}, &stdout, &stderr)

			// Then
			if !errors.Is(err, ErrExitFalse) {
				t.Fatalf("expected unsupported date feature to return ErrExitFalse, got %v", err)
			}
			if got := stdout.String(); got != "" {
				t.Fatalf("expected empty stdout, got %q", got)
			}
			assertDateDiagnostic(t, stderr.String(), "unsupported")
		})
	}
}

func TestDateApplet_returnsErrExitFalseAndDiagnostic_whenRunWithUnsupportedInput(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		diagnostic   string
		exactMessage bool
	}{
		{name: "non epoch date", args: []string{"-u", "-d", "1970-01-01", "+%Y"}, diagnostic: "date:"},
		{name: "bad epoch", args: []string{"-u", "-d", "@bad", "+%s"}, diagnostic: "date:"},
		{name: "unsupported format token", args: []string{"-u", "-d", "@0", "+%Q"}, diagnostic: "date:"},
		{name: "dangling percent", args: []string{"-u", "-d", "@0", "+%"}, diagnostic: "date: unsupported format: %\n", exactMessage: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			applet := newDateApplet()
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			// When
			err := applet.Run(context.Background(), tt.args, &bytes.Buffer{}, &stdout, &stderr)

			// Then
			if !errors.Is(err, ErrExitFalse) {
				t.Fatalf("expected unsupported date input to return ErrExitFalse, got %v", err)
			}
			if got := stdout.String(); got != "" {
				t.Fatalf("expected empty stdout, got %q", got)
			}
			if tt.exactMessage {
				if got := stderr.String(); got != tt.diagnostic {
					t.Fatalf("expected exact date diagnostic %q, got %q", tt.diagnostic, got)
				}
				return
			}
			assertDateDiagnostic(t, stderr.String(), tt.diagnostic)
		})
	}
}

func assertDateDiagnostic(t *testing.T, stderr string, want string) {
	t.Helper()
	if !strings.Contains(stderr, want) {
		t.Fatalf("expected date diagnostic to contain %q, got %q", want, stderr)
	}
	if strings.Count(stderr, "\n") != 1 {
		t.Fatalf("expected one-line date diagnostic, got %q", stderr)
	}
}
