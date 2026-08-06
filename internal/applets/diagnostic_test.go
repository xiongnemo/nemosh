package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// The shell prints "<applet>: <error>", so an applet's error text must not
// repeat the applet name. What it must not do either is leak the resolved host
// path or Windows phrasing: busybox names the operand exactly as the user wrote
// it and spells the cause the way strerror does.
func TestAppletDiagnostics_nameTheOperandAsWritten_whenTheFileIsMissing(t *testing.T) {
	tests := []struct {
		applet string
		args   []string
		want   string
	}{
		// open_or_warn, libbb/xfuncs_printf.c:169
		{applet: "cat", want: "can't open 'nope.txt': No such file or directory"},
		{applet: "tail", want: "can't open 'nope.txt': No such file or directory"},
		// fopen_or_warn -> bb_simple_perror_msg, libbb/wfopen.c:11
		{applet: "head", want: "nope.txt: No such file or directory"},
		{applet: "wc", want: "nope.txt: No such file or directory"},
		// findutils/grep.c:671
		{applet: "grep", args: []string{"pattern"}, want: "nope.txt: No such file or directory"},
	}
	for _, tt := range tests {
		t.Run(tt.applet, func(t *testing.T) {
			// Given
			applet, ok := applets.DefaultRegistry.Lookup(tt.applet)
			if !ok {
				t.Fatalf("expected %s applet to be registered", tt.applet)
			}
			ctx := applets.WithProcessView(context.Background(), diagnosticTestView{cwd: t.TempDir()})
			var stdout, stderr bytes.Buffer

			// When
			err := applet.Run(ctx, append(tt.args, "nope.txt"), &bytes.Buffer{}, &stdout, &stderr)

			// Then
			if err == nil {
				t.Fatalf("expected %s to fail on a missing operand", tt.applet)
			}
			if got := err.Error(); got != tt.want {
				t.Fatalf("expected %s diagnostic %q, got %q", tt.applet, tt.want, got)
			}
		})
	}
}

// The operand is echoed verbatim, so a relative operand stays relative. This is
// what makes a diagnostic reproducible: the resolved host path names a machine.
func TestAppletDiagnostics_keepARelativeOperandRelative(t *testing.T) {
	// Given
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatalf("expected the fixture directory to be created, got %v", err)
	}
	applet, ok := applets.DefaultRegistry.Lookup("cat")
	if !ok {
		t.Fatal("expected cat applet to be registered")
	}
	ctx := applets.WithProcessView(context.Background(), diagnosticTestView{cwd: dir})
	var stdout, stderr bytes.Buffer

	// When
	err := applet.Run(ctx, []string{"sub/nope.txt"}, &bytes.Buffer{}, &stdout, &stderr)

	// Then
	if err == nil {
		t.Fatal("expected cat to fail on a missing nested operand")
	}
	if got, want := err.Error(), "can't open 'sub/nope.txt': No such file or directory"; got != want {
		t.Fatalf("expected cat diagnostic %q, got %q", want, got)
	}
}

type diagnosticTestView struct{ cwd string }

func (v diagnosticTestView) WorkingDirectory() string        { return v.cwd }
func (v diagnosticTestView) Environ() []string               { return nil }
func (v diagnosticTestView) LookupEnv(string) (string, bool) { return "", false }
func (v diagnosticTestView) ResolvePath(path string) string  { return filepath.Join(v.cwd, path) }
