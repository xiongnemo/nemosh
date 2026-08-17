package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// An option these applets do not implement used to be handed to the file opener,
// so `cat -b f` reported `cat: cannot open '-b': No such file or directory`. The
// failure was loud but named the wrong cause, sending the reader after a file
// that was never meant to be one.
//
// `cat -n`, `head -c` and now `tail -c` were on this list until they were
// implemented, which is the right way for a row to leave it -- the asymmetry
// between head and tail was documented as deliberate for exactly as long as it
// took to implement the missing half.
//
// `tail -f` stays: following a file needs a polling loop and a decision about
// what to do when it is truncated or replaced, and an implementation that
// silently stops following is worse than one that says it cannot.
func TestStreamApplets_refuseAnUnknownOptionByName(t *testing.T) {
	for _, test := range []struct {
		applet string
		args   []string
	}{
		{applet: "cat", args: []string{"-b"}},
		{applet: "cat", args: []string{"-A", "f.txt"}},
		{applet: "head", args: []string{"-q", "f.txt"}},
		{applet: "tail", args: []string{"-f", "f.txt"}},
	} {
		t.Run(test.applet+" "+strings.Join(test.args, " "), func(t *testing.T) {
			// Given
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a\nb\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			applet, ok := applets.DefaultRegistry.Lookup(test.applet)
			if !ok {
				t.Fatalf("%s is not registered", test.applet)
			}
			var stdout, stderr bytes.Buffer
			ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: dir})

			// When
			err := applet.Run(ctx, test.args, strings.NewReader(""), &stdout, &stderr)

			// Then
			if err == nil {
				t.Fatalf("%s %v succeeded, want a refusal", test.applet, test.args)
			}
			message := stderr.String() + err.Error()
			if strings.Contains(message, "No such file") || strings.Contains(message, "cannot open") {
				t.Fatalf("%s %v still reports an option as a missing file: %q", test.applet, test.args, message)
			}
			if !strings.Contains(message, strings.TrimPrefix(test.args[0], "-")) {
				t.Fatalf("%s %v reported %q, want it to name the option", test.applet, test.args, message)
			}
			if stdout.String() != "" {
				t.Fatalf("%s %v wrote %q before refusing", test.applet, test.args, stdout.String())
			}
		})
	}
}

// What already worked has to keep working, including the operands that only
// look like options.
func TestStreamApplets_keepTheirSupportedForms(t *testing.T) {
	for _, test := range []struct {
		name   string
		applet string
		args   []string
		want   string
	}{
		{name: "cat reads a file", applet: "cat", args: []string{"f.txt"}, want: "a\nb\nc\n"},
		{name: "head defaults to ten lines", applet: "head", args: []string{"f.txt"}, want: "a\nb\nc\n"},
		{name: "head -n limits", applet: "head", args: []string{"-n", "2", "f.txt"}, want: "a\nb\n"},
		{name: "head -n 0 prints nothing", applet: "head", args: []string{"-n", "0", "f.txt"}, want: ""},
		{name: "tail -n limits", applet: "tail", args: []string{"-n", "1", "f.txt"}, want: "c\n"},
		{name: "a -- ends the options", applet: "cat", args: []string{"--", "f.txt"}, want: "a\nb\nc\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a\nb\nc\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			applet, _ := applets.DefaultRegistry.Lookup(test.applet)
			var stdout, stderr bytes.Buffer
			ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: dir})

			// When
			err := applet.Run(ctx, test.args, strings.NewReader(""), &stdout, &stderr)

			// Then
			if err != nil {
				t.Fatalf("%s %v: %v (stderr %q)", test.applet, test.args, err, stderr.String())
			}
			if stdout.String() != test.want {
				t.Fatalf("%s %v stdout = %q, want %q", test.applet, test.args, stdout.String(), test.want)
			}
		})
	}
}

// `-n` with nothing after it is a usage error, not a file named "-n".
func TestHeadTail_refuseALineCountWithoutItsValue(t *testing.T) {
	for _, name := range []string{"head", "tail"} {
		t.Run(name, func(t *testing.T) {
			// Given
			applet, _ := applets.DefaultRegistry.Lookup(name)
			var stdout, stderr bytes.Buffer
			ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: t.TempDir()})

			// When
			err := applet.Run(ctx, []string{"-n"}, strings.NewReader(""), &stdout, &stderr)

			// Then
			if err == nil {
				t.Fatalf("%s -n succeeded, want a refusal", name)
			}
			if message := stderr.String() + err.Error(); strings.Contains(message, "No such file") {
				t.Fatalf("%s -n reports a missing file: %q", name, message)
			}
		})
	}
}
