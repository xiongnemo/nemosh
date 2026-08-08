package runtime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `set -o nocaseglob` makes pathname expansion case-insensitive, which busybox
// implements as FNM_CASEFOLD (shell/ash.c:9230). It matters more on Windows
// than elsewhere: NTFS is case-insensitive, so a pattern that fails only
// because of case is surprising here in a way it is not on a Unix filesystem.
func TestNoCaseGlob(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "off by default, so an unmatched pattern stays literal",
			script: "cd $TMPDIR_FIXTURE\necho upper.*\n",
			want:   "upper.*\n",
		},
		{
			name:   "on, the pattern matches whatever the case",
			script: "cd $TMPDIR_FIXTURE\nset -o nocaseglob\necho upper.*\n",
			want:   "UPPER.TXT\n",
		},
		{
			name:   "and the other direction too",
			script: "cd $TMPDIR_FIXTURE\nset -o nocaseglob\necho LOWER.*\n",
			want:   "lower.txt\n",
		},
		{
			name:   "turning it back off restores the literal",
			script: "cd $TMPDIR_FIXTURE\nset -o nocaseglob\nset +o nocaseglob\necho upper.*\n",
			want:   "upper.*\n",
		},
		{
			name:   "an exact-case match works either way",
			script: "cd $TMPDIR_FIXTURE\necho lower.*\n",
			want:   "lower.txt\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			dir := noCaseGlobFixture(t)

			// When
			status, stdout, stderr := runSetScript(t, "TMPDIR_FIXTURE="+dir+"\n"+test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// The option shows up in the listing, so `set -o` says what is on.
func TestNoCaseGlob_isListed(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "set -o nocaseglob\nset -o\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if !containsLine(stdout, "nocaseglob") {
		t.Fatalf("set -o output %q does not mention nocaseglob", stdout)
	}
}

func noCaseGlobFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"UPPER.TXT", "lower.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.ToSlash(dir)
}

func containsLine(text, want string) bool {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, want) {
			return true
		}
	}
	return false
}
