//go:build windows

package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// The clipboard is one machine-wide slot, so these tests borrow it and hand it
// back byte for byte -- the raw form, so that a lone CR in whatever the user
// had copied survives the round trip. borrowClipboard skips rather than
// clobbers when the clipboard holds something with no text in it at all, an
// image say, because there would be no way to put that back afterwards.
func borrowClipboard(t *testing.T) {
	t.Helper()
	if !clipboardHoldsTextOrNothing(t) {
		t.Skip("clipboard holds a non-text format; refusing to overwrite it")
	}
	saved, err := readClipboardTextRaw()
	if err != nil {
		t.Skipf("clipboard unavailable: %v", err)
	}
	t.Cleanup(func() {
		if err := writeClipboardTextRaw(saved); err != nil {
			t.Errorf("restore clipboard: %v", err)
		}
	})
}

// An empty clipboard reports no formats at all; anything else that offers no
// CF_UNICODETEXT is data this test cannot reproduce.
func clipboardHoldsTextOrNothing(t *testing.T) bool {
	t.Helper()
	holds := false
	if err := withClipboard(func() error {
		formats, _, _ := procCountClipboardFormats.Call()
		if formats == 0 {
			holds = true
			return nil
		}
		available, _, _ := procIsClipboardFormatAvailable.Call(cfUnicodeText)
		holds = available != 0
		return nil
	}); err != nil {
		t.Skipf("clipboard unavailable: %v", err)
	}
	return holds
}

func TestRuntime_readsWhatItWrote_whenBothEndsAreDevClipboard(t *testing.T) {
	// Given
	borrowClipboard(t)
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "echo hello > /dev/clipboard\ncat /dev/clipboard\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d with stderr %q", status, stderr.String())
	}
	if got := stdout.String(); got != "hello\n" {
		t.Fatalf("expected clipboard round trip %q, got %q", "hello\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

// Another Windows program pastes CRLF, so that is what has to be on the
// clipboard even though the script only ever wrote LF.
func TestClipboard_storesCarriageReturnLineFeed_whenTheScriptWroteLineFeed(t *testing.T) {
	// Given
	borrowClipboard(t)
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "printf 'a\\nb\\n' > /dev/clipboard\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d with stderr %q", status, stderr.String())
	}
	stored, err := readClipboardTextRaw()
	if err != nil {
		t.Fatalf("read clipboard: %v", err)
	}
	if stored != "a\r\nb\r\n" {
		t.Fatalf("expected stored clipboard %q, got %q", "a\r\nb\r\n", stored)
	}
}

// Appending reads the clipboard back first; truncating does not.
func TestRuntime_appendsToTheClipboard_whenTheRedirectIsAppend(t *testing.T) {
	// Given
	borrowClipboard(t)
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "echo one > /dev/clipboard\necho two >> /dev/clipboard\ncat /dev/clipboard\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d with stderr %q", status, stderr.String())
	}
	if got := stdout.String(); got != "one\ntwo\n" {
		t.Fatalf("expected appended clipboard %q, got %q", "one\ntwo\n", got)
	}
}

// UTF-8 in, UTF-8 out: the clipboard itself is UTF-16, and the boundary hides
// that (docs/design/windows-path-model.md:241).
func TestRuntime_carriesNonASCIIText_throughDevClipboard(t *testing.T) {
	// Given
	borrowClipboard(t)
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "echo 你好世界 > /dev/clipboard\ncat /dev/clipboard\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d with stderr %q", status, stderr.String())
	}
	if got := stdout.String(); got != "你好世界\n" {
		t.Fatalf("expected non-ASCII round trip %q, got %q", "你好世界\n", got)
	}
}
