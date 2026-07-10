package applets_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestDefaultRegistry_selectsCharacters_whenCutRuns(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("cut")
	if !ok {
		t.Fatal("expected cut applet to be registered")
	}
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-c", "2-4"}, strings.NewReader("abcdef\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cut to succeed, got %v", err)
	}
	if got, want := stdout.String(), "bcd\n"; got != want {
		t.Fatalf("expected cut output %q, got %q", want, got)
	}
}
