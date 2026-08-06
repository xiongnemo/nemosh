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

// The mutating applets leak the same way the readers did. busybox spells each of
// these differently -- quoted behind a verb, quoted bare, or unquoted -- so we
// follow it applet by applet instead of inventing one uniform message.
func TestAppletDiagnostics_nameTheOperandAsWritten_whenTheOperationFails(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(t *testing.T, dir string)
		applet string
		args   []string
		want   string
	}{
		{
			name:   "rm on a missing file", // libbb/remove_file.c:23
			applet: "rm", args: []string{"nope.txt"},
			want: "can't remove 'nope.txt': No such file or directory",
		},
		{
			name:   "mkdir over an existing directory", // libbb/make_directory.c:150
			setup:  func(t *testing.T, dir string) { makeFixtureDir(t, dir, "taken") },
			applet: "mkdir", args: []string{"taken"},
			want: "can't create directory 'taken': File exists",
		},
		{
			name:   "rmdir on a missing directory", // coreutils/rmdir.c:78, "Match gnu rmdir msg"
			applet: "rmdir", args: []string{"nope"},
			want: "'nope': No such file or directory",
		},
		{
			// Go folds ERROR_DIR_NOT_EMPTY into fs.ErrExist, so the portable
			// sentinels alone would call this "File exists".
			name: "rmdir on a directory that still has children",
			setup: func(t *testing.T, dir string) {
				makeFixtureDir(t, dir, "full")
				makeFixtureFile(t, dir, filepath.Join("full", "child.txt"))
			},
			applet: "rmdir", args: []string{"full"},
			want: "'full': Directory not empty",
		},
		{
			name:   "touch below a missing directory", // coreutils/touch.c:188
			applet: "touch", args: []string{"nope/x.txt"},
			want: "nope/x.txt: No such file or directory",
		},
		{
			name:   "chmod on a missing file", // coreutils/chmod.c:102
			applet: "chmod", args: []string{"644", "nope.txt"},
			want: "nope.txt: No such file or directory",
		},
		{
			name:   "find on a missing root", // libbb/recursive_action.c:158
			applet: "find", args: []string{"nope"},
			want: "nope: No such file or directory",
		},
		{
			name:   "ls on a missing operand", // coreutils/ls.c:819
			applet: "ls", args: []string{"nope"},
			want: "nope: No such file or directory",
		},
		{
			name:   "cp from a missing source", // libbb/copy_file.c:98
			applet: "cp", args: []string{"nope.txt", "out.txt"},
			want: "can't stat 'nope.txt': No such file or directory",
		},
		{
			name:   "cp into a missing directory", // libbb/copy_file.c:64
			setup:  func(t *testing.T, dir string) { makeFixtureFile(t, dir, "a.txt") },
			applet: "cp", args: []string{"a.txt", "nope/out.txt"},
			want: "can't create 'nope/out.txt': No such file or directory",
		},
		{
			// cp a.txt d writes d/a.txt, and the diagnostic names that joined
			// path, not the bare destination operand.
			name: "cp onto a name already taken by a directory",
			setup: func(t *testing.T, dir string) {
				makeFixtureFile(t, dir, "a.txt")
				makeFixtureDir(t, dir, "d")
				makeFixtureDir(t, dir, filepath.Join("d", "a.txt"))
			},
			applet: "cp", args: []string{"a.txt", "d"},
			want: "can't create 'd/a.txt': Is a directory",
		},
		{
			name:   "mv from a missing source", // coreutils/mv.c:143
			applet: "mv", args: []string{"nope.txt", "out.txt"},
			want: "can't rename 'nope.txt': No such file or directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			dir := t.TempDir()
			if tt.setup != nil {
				tt.setup(t, dir)
			}
			applet, ok := applets.DefaultRegistry.Lookup(tt.applet)
			if !ok {
				t.Fatalf("expected %s applet to be registered", tt.applet)
			}
			ctx := applets.WithProcessView(context.Background(), diagnosticTestView{cwd: dir})
			var stdout, stderr bytes.Buffer

			// When
			err := applet.Run(ctx, tt.args, &bytes.Buffer{}, &stdout, &stderr)

			// Then
			if err == nil {
				t.Fatalf("expected %s %v to fail", tt.applet, tt.args)
			}
			if got := err.Error(); got != tt.want {
				t.Fatalf("expected %s diagnostic %q, got %q", tt.applet, tt.want, got)
			}
		})
	}
}

func makeFixtureDir(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.Mkdir(filepath.Join(dir, name), 0o700); err != nil {
		t.Fatalf("expected fixture directory %s to be created, got %v", name, err)
	}
}

func makeFixtureFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0o600); err != nil {
		t.Fatalf("expected fixture file %s to be created, got %v", name, err)
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
