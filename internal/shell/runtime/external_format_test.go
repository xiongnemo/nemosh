package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasWindowsExecutableSuffixOrDot_blocksTheSuffixSearch(t *testing.T) {
	for _, testCase := range []struct {
		name string
		want bool
	}{
		{name: `C:\tools\run.exe`, want: true},
		{name: `C:\tools\run.COM`, want: true},
		{name: `C:\tools\run.Sh`, want: true},
		{name: `C:\tools\run.bat`, want: true},
		{name: `C:\tools\run.cmd`, want: true},
		// Windows drops a trailing dot when opening, so appending to it would
		// name a different file than the one the user asked for.
		{name: `C:\tools\run.`, want: true},
		{name: `C:\tools\run`, want: false},
		{name: `C:\tools\run.txt`, want: false},
		{name: `C:\tools\run.batch`, want: false},
		{name: `C:\exe\run`, want: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := hasWindowsExecutableSuffixOrDot(testCase.name); got != testCase.want {
				t.Fatalf("hasWindowsExecutableSuffixOrDot(%q) = %v, want %v", testCase.name, got, testCase.want)
			}
		})
	}
}

func TestHasExecutableFormat_acceptsOnlyScriptsAndImages(t *testing.T) {
	dir := t.TempDir()
	for _, testCase := range []struct {
		name    string
		content string
		want    bool
	}{
		{name: "script", content: "#!/bin/sh\necho hi\n", want: true},
		{name: "image", content: "MZ\x90\x00rest of the header", want: true},
		{name: "text", content: "just words\n"},
		// busybox refuses to judge a file shorter than four bytes.
		{name: "tiny", content: "#!"},
		{name: "empty", content: ""},
		// A DLL opens with MZ but is never runnable, so it is excluded by name.
		{name: "library.dll", content: "MZ\x90\x00rest of the header"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			path := filepath.Join(dir, testCase.name)
			if err := os.WriteFile(path, []byte(testCase.content), 0o600); err != nil {
				t.Fatalf("write %s: %v", testCase.name, err)
			}

			got, err := hasExecutableFormat(path)

			if err != nil {
				t.Fatalf("hasExecutableFormat(%q): %v", testCase.name, err)
			}
			if got != testCase.want {
				t.Fatalf("hasExecutableFormat(%q) = %v, want %v", testCase.name, got, testCase.want)
			}
		})
	}
}
