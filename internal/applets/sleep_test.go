package applets

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSleepApplet_returnsQuickly_whenDurationIsZero(t *testing.T) {
	// Given
	applet := newSleepApplet()
	started := time.Now()

	// When
	err := applet.Run(context.Background(), []string{"0"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected sleep 0 to succeed, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("expected sleep 0 to return quickly, took %s", elapsed)
	}
}

func TestSleepApplet_succeeds_whenDoubleDashPrecedesZero(t *testing.T) {
	// Given
	applet := newSleepApplet()

	// When
	err := applet.Run(context.Background(), []string{"--", "0"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected sleep -- 0 to succeed, got %v", err)
	}
}

func TestSleepApplet_acceptsMultipleZeroSuffixDurations(t *testing.T) {
	// Given
	applet := newSleepApplet()
	args := []string{"0", "0s", "0m", "0h", "0d", "0.0s"}

	// When
	err := applet.Run(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected multiple zero durations to succeed, got %v", err)
	}
}

func TestSleepApplet_returnsErrExitFalseAndDiagnostic_whenOperandIsMissing(t *testing.T) {
	// Given
	applet := newSleepApplet()
	tests := []struct {
		name string
		args []string
	}{
		{name: "nil args", args: nil},
		{name: "double dash only", args: []string{"--"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer

			// When
			err := applet.Run(context.Background(), tt.args, &bytes.Buffer{}, &bytes.Buffer{}, &stderr)

			// Then
			if !errors.Is(err, ErrExitFalse) {
				t.Fatalf("expected missing sleep operand to return ErrExitFalse, got %v", err)
			}
			if got := strings.TrimSpace(stderr.String()); got == "" || strings.Contains(got, "\n") {
				t.Fatalf("expected concise one-line diagnostic, got %q", stderr.String())
			}
		})
	}
}

func TestSleepApplet_returnsErrExitFalseAndDiagnostic_whenOperandIsInvalid(t *testing.T) {
	// Given
	applet := newSleepApplet()
	tests := []string{"abc", "1x", "1.2.3", "ms", "-1", "9223372036.854776s"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			var stderr bytes.Buffer
			started := time.Now()

			// When
			err := applet.Run(context.Background(), []string{input}, &bytes.Buffer{}, &bytes.Buffer{}, &stderr)

			// Then
			if !errors.Is(err, ErrExitFalse) {
				t.Fatalf("expected invalid sleep operand %q to return ErrExitFalse, got %v", input, err)
			}
			if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
				t.Fatalf("expected invalid sleep operand %q to fail quickly, took %s", input, elapsed)
			}
			if got := strings.TrimSpace(stderr.String()); got == "" || strings.Contains(got, "\n") {
				t.Fatalf("expected concise one-line diagnostic for %q, got %q", input, stderr.String())
			}
		})
	}
}

func TestSleepApplet_stopsWhenContextIsCancelled_whenDurationIsPending(t *testing.T) {
	// Given
	applet := newSleepApplet()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	// When
	go func() {
		done <- applet.Run(ctx, []string{"1h"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})
	}()
	cancel()

	// Then
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected canceled context, got %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected sleep to stop promptly after context cancellation")
	}
}

func TestSleepParser_returnsDuration_whenOperandIsValid(t *testing.T) {
	// Given
	tests := []struct {
		name string
		arg  string
		want time.Duration
	}{
		{name: "zero seconds suffix", arg: "0s", want: 0},
		{name: "one second suffix", arg: "1s", want: time.Second},
		{name: "one minute suffix", arg: "1m", want: time.Minute},
		{name: "one hour suffix", arg: "1h", want: time.Hour},
		{name: "one day suffix", arg: "1d", want: 24 * time.Hour},
		{name: "fractional seconds without suffix", arg: "0.001", want: time.Millisecond},
		{name: "fractional seconds suffix", arg: "0.5s", want: 500 * time.Millisecond},
		{name: "fractional minutes suffix", arg: "1.5m", want: 90 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got, err := parseSleepDuration(tt.arg)

			// Then
			if err != nil {
				t.Fatalf("expected %q to parse, got %v", tt.arg, err)
			}
			if got != tt.want {
				t.Fatalf("expected %q to parse as %s, got %s", tt.arg, tt.want, got)
			}
		})
	}
}
