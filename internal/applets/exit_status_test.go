package applets_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// cut, sort and uniq were all written to fail with status 2. Only sort is
// entitled to it: sort_main opens with `xfunc_error_retval = 2`
// (coreutils/sort.c:468), so every later bb_show_usage and every xfopen_stdin
// death carries that status. cut and uniq never touch xfunc_error_retval, so
// they get the EXIT_FAILURE default from libbb/default_error_retval.c:16 -- cut
// by assigning it explicitly to its retval, uniq by dying through xopen.
//
// The status is what a script branches on, so a wrong one is not cosmetic.
func TestAppletExitStatus_matchTheReferenceFailureStatus(t *testing.T) {
	tests := []struct {
		name   string
		applet string
		args   []string
		want   int
	}{
		// coreutils/cut.c: `retval = EXIT_FAILURE` after fopen_or_warn_stdin.
		{name: "cut on a missing file", applet: "cut", args: []string{"-c", "1", "nope.txt"}, want: 1},
		// getopt32 -> bb_show_usage -> xfunc_die -> exit(xfunc_error_retval).
		{name: "cut on an unknown option", applet: "cut", args: []string{"-x"}, want: 1},
		{name: "cut with no list", applet: "cut", args: nil, want: 1},
		// bb_error_msg_and_die, coreutils/cut.c:341 and :386.
		{name: "cut with a delimiter but no fields", applet: "cut", args: []string{"-d", ":", "-c", "1"}, want: 1},
		{name: "cut on a zero range", applet: "cut", args: []string{"-c", "0"}, want: 1},

		// coreutils/uniq.c:76 opens through xopen; :81 shows usage. Neither
		// raises xfunc_error_retval, so both land on the default.
		{name: "uniq on a missing file", applet: "uniq", args: []string{"nope.txt"}, want: 1},
		{name: "uniq on an unknown option", applet: "uniq", args: []string{"-x"}, want: 1},
		{name: "uniq with a second operand", applet: "uniq", args: []string{"a.txt", "b.txt"}, want: 1},

		// sort keeps its 2; these rows are the reason the others are not 2.
		{name: "sort on a missing file", applet: "sort", args: []string{"nope.txt"}, want: 2},
		{name: "sort on an unknown option", applet: "sort", args: []string{"-x"}, want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			applet, ok := applets.DefaultRegistry.Lookup(tt.applet)
			if !ok {
				t.Fatalf("expected %s applet to be registered", tt.applet)
			}
			ctx := applets.WithProcessView(context.Background(), diagnosticTestView{cwd: t.TempDir()})
			var stdout, stderr bytes.Buffer

			// When
			err := applet.Run(ctx, tt.args, &bytes.Buffer{}, &stdout, &stderr)

			// Then
			status, ok := applets.StatusCode(err)
			if !ok {
				t.Fatalf("expected %s %v to fail with an exit status, got %v", tt.applet, tt.args, err)
			}
			if status != tt.want {
				t.Fatalf("expected %s %v to exit %d, got %d", tt.applet, tt.args, tt.want, status)
			}
		})
	}
}

// busybox's uniq opens its input with xopen, which reports a failure as
// "cannot open '%s'" (libbb/xfuncs_printf.c:151). cut and sort go through
// fopen_or_warn_stdin instead (libbb/wfopen_input.c:16), whose
// bb_simple_perror_msg prints the operand bare. Three readers, two shapes.
func TestAppletExitStatus_uniqQuotesTheOperandTheWayXopenDoes(t *testing.T) {
	// Given
	applet, ok := applets.DefaultRegistry.Lookup("uniq")
	if !ok {
		t.Fatal("expected uniq applet to be registered")
	}
	ctx := applets.WithProcessView(context.Background(), diagnosticTestView{cwd: t.TempDir()})
	var stdout, stderr bytes.Buffer

	// When
	err := applet.Run(ctx, []string{"nope.txt"}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	if err == nil {
		t.Fatal("expected uniq to fail on a missing operand")
	}
	want := "uniq: cannot open 'nope.txt': No such file or directory\n"
	if got := stderr.String(); got != want {
		t.Fatalf("expected uniq to write %q to stderr, got %q", want, got)
	}
}

// coreutils/cut.c holds retval at EXIT_FAILURE and `continue`s, so a missing
// operand does not stop the operands after it from being cut. sort is the
// opposite and says so: "coreutils 6.9 compat: abort on first open error"
// (coreutils/sort.c:566).
func TestAppletExitStatus_cutKeepsGoingAfterAnUnreadableOperand(t *testing.T) {
	// Given
	dir := t.TempDir()
	makeFixtureFile(t, dir, "after.txt")
	applet, ok := applets.DefaultRegistry.Lookup("cut")
	if !ok {
		t.Fatal("expected cut applet to be registered")
	}
	ctx := applets.WithProcessView(context.Background(), diagnosticTestView{cwd: dir})
	var stdout, stderr bytes.Buffer

	// When
	err := applet.Run(ctx, []string{"-c", "1", "nope.txt", "after.txt"}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	if status, ok := applets.StatusCode(err); !ok || status != 1 {
		t.Fatalf("expected cut to exit 1, got status=%d ok=%v err=%v", status, ok, err)
	}
	if got, want := stdout.String(), "x\n"; got != want {
		t.Fatalf("expected cut to still cut %s, wrote %q to stdout, want %q",
			filepath.Join(dir, "after.txt"), got, want)
	}
	want := "cut: nope.txt: No such file or directory\n"
	if got := stderr.String(); got != want {
		t.Fatalf("expected cut to write %q to stderr, got %q", want, got)
	}
}
