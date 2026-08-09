package applets_test

import (
	"bytes"
	"context"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func runUname(t *testing.T, args ...string) string {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup("uname")
	if !ok {
		t.Fatal("uname is not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: t.TempDir()})
	if err := applet.Run(ctx, args, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("uname %v: %v (stderr %q)", args, err, stderr.String())
	}
	return strings.TrimSuffix(stdout.String(), "\n")
}

// Release and version were hardcoded to "unknown" -- the applet never asked the
// operating system. busybox-w32 reports major.minor and the build number
// (win32/uname.c:21-23), so `uname -a` there reads `10.0 19045` where Nemosh
// read `unknown unknown`.
func TestUname_reportsTheRealReleaseAndVersion(t *testing.T) {
	// Windows is where these have a source. Elsewhere the applet says unknown
	// on purpose, because Go exposes no portable way to read them and this
	// platform is build-and-test rather than supported.
	if runtime.GOOS != "windows" {
		t.Skip("release and version are only sourced on Windows")
	}

	// When
	release := runUname(t, "-r")
	version := runUname(t, "-v")

	// Then
	if !regexp.MustCompile(`^\d+\.\d+$`).MatchString(release) {
		t.Errorf("uname -r = %q, want major.minor", release)
	}
	if !regexp.MustCompile(`^\d+$`).MatchString(version) {
		t.Errorf("uname -v = %q, want a build number", version)
	}
}

// `uname -a` carries them in place, so the middle of the line stops reading
// "unknown unknown".
func TestUname_allCarriesReleaseAndVersion(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("release and version are only sourced on Windows")
	}

	// When
	all := runUname(t, "-a")

	// Then
	fields := strings.Fields(all)
	if len(fields) < 5 {
		t.Fatalf("uname -a = %q, want at least five fields", all)
	}
	if fields[2] == "unknown" || fields[3] == "unknown" {
		t.Fatalf("uname -a = %q, want the release and version filled in", all)
	}
	if fields[2] != runUname(t, "-r") || fields[3] != runUname(t, "-v") {
		t.Fatalf("uname -a = %q disagrees with -r and -v", all)
	}
}

// Processor and hardware platform stay unknown. busybox answers the same, and
// inventing a value would be worse than saying it is not known.
func TestUname_leavesWhatItCannotAnswerUnknown(t *testing.T) {
	for _, option := range []string{"-p", "-i"} {
		if got := runUname(t, option); got != "unknown" {
			t.Errorf("uname %s = %q, want unknown", option, got)
		}
	}
}
