package applets_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestDefaultRegistry_XargsRunsDefaultEcho_whenInputHasWords(t *testing.T) {
	// Given
	applet := lookupXargsApplet(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, strings.NewReader("alpha beta\ngamma\t"), &stdout, &stderr)

	// Then
	if err != nil {
		t.Fatalf("expected xargs to succeed, got %v", err)
	}
	if got := stdout.String(); got != "alpha beta gamma\n" {
		t.Fatalf("expected xargs output %q, got %q", "alpha beta gamma\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestDefaultRegistry_XargsAppendsInputWords_whenCommandAndArgsProvided(t *testing.T) {
	// Given
	applet := lookupXargsApplet(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"echo", "prefix"}, strings.NewReader("one\ntwo\n"), &stdout, &stderr)

	// Then
	if err != nil {
		t.Fatalf("expected xargs to succeed, got %v", err)
	}
	if got := stdout.String(); got != "prefix one two\n" {
		t.Fatalf("expected xargs output %q, got %q", "prefix one two\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestDefaultRegistry_XargsRunsDefaultEchoOnce_whenInputIsEmpty(t *testing.T) {
	// Given
	applet := lookupXargsApplet(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, strings.NewReader(""), &stdout, &stderr)

	// Then
	if err != nil {
		t.Fatalf("expected xargs to succeed, got %v", err)
	}
	if got := stdout.String(); got != "\n" {
		t.Fatalf("expected xargs output %q, got %q", "\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestDefaultRegistry_XargsReturnsFalse_whenChildAppletReturnsFalse(t *testing.T) {
	// Given
	applet := lookupXargsApplet(t)

	// When
	err := applet.Run(context.Background(), []string{"false"}, strings.NewReader("one\n"), &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if !errors.Is(err, applets.ErrExitFalse) {
		t.Fatalf("expected false sentinel, got %v", err)
	}
}

// -0 is implemented now, and it is the option that matters most: NUL separation
// is the only way a filename with a blank in it survives the trip, and splitting
// on whitespace -- which is what this did for every input -- is how `xargs rm`
// comes to delete the wrong thing.
func TestDefaultRegistry_XargsSplitsOnNul(t *testing.T) {
	// Given
	applet := lookupXargsApplet(t)
	var stdout, stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-0", "echo"},
		strings.NewReader("a b\x00c\x00"), &stdout, &stderr)

	// Then
	if err != nil {
		t.Fatalf("xargs -0: %v", err)
	}
	if stdout.String() != "a b c\n" {
		t.Fatalf("xargs -0 = %q, want the blank kept inside one argument", stdout.String())
	}
}

// An option xargs still does not have is refused by name rather than treated as
// the command to run, which is what would happen if the parser simply stopped at
// the first dash.
func TestDefaultRegistry_XargsRejectsAnOptionItDoesNotHave(t *testing.T) {
	// Given
	applet := lookupXargsApplet(t)
	var stdout, stderr bytes.Buffer

	// When: -P is GNU's parallel option, which this has no way to honour
	err := applet.Run(context.Background(), []string{"-P4", "echo"}, strings.NewReader("one\n"), &stdout, &stderr)

	// Then
	if err == nil {
		t.Fatal("expected an unsupported option error")
	}
}

func lookupXargsApplet(t *testing.T) applets.Applet {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup("xargs")
	if !ok {
		t.Fatal("expected xargs applet to be registered")
	}
	return applet
}
