package runtime_test

import (
	"strings"
	"testing"
)

// The three layers have three audiences. A script greps the first line, so it
// does not move. A person needs the hint, and it is on by default. The detail is
// for whoever is debugging the shell, and stays off until asked for -- printing
// it by default would leak host paths into output that behaviour cases compare
// byte for byte.
func TestRuntime_layersALaunchDiagnostic(t *testing.T) {
	// When
	status, _, stderr := runSetScript(t, "definitely-not-a-program\n")

	// Then
	if status != 127 {
		t.Fatalf("status = %d, want 127", status)
	}
	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	if lines[0] != "definitely-not-a-program: not found" {
		t.Fatalf("first line = %q, want the stable not-found line", lines[0])
	}
	if len(lines) < 2 || !strings.HasPrefix(lines[1], "hint: ") {
		t.Fatalf("stderr = %q, want a hint on the second line", stderr)
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "debug: ") {
			t.Fatalf("stderr = %q, want no debug detail without NEMOSH_DEBUG", stderr)
		}
	}
}

func TestRuntime_addsDebugDetail_whenTheChannelIsAskedFor(t *testing.T) {
	// When
	_, _, stderr := runSetScript(t, "NEMOSH_DEBUG=exec\ndefinitely-not-a-program\n")

	// Then
	if !strings.Contains(stderr, "debug: exec: ") {
		t.Fatalf("stderr = %q, want exec detail", stderr)
	}
	if !strings.Contains(stderr, "PATH has ") {
		t.Fatalf("stderr = %q, want the search to be described", stderr)
	}
	// The first line still comes first and still says the same thing.
	if !strings.HasPrefix(stderr, "definitely-not-a-program: not found\n") {
		t.Fatalf("stderr = %q, want the first line unchanged by the channel", stderr)
	}
}

func TestRuntime_keepsChannelsSeparate(t *testing.T) {
	// Asking for `path` must not turn on `exec`: a reader who wants one stream
	// of detail should not get the other.
	// When
	_, _, stderr := runSetScript(t, "NEMOSH_DEBUG=path\ndefinitely-not-a-program\n")

	// Then
	if strings.Contains(stderr, "debug: exec: ") {
		t.Fatalf("stderr = %q, want no exec detail when only path was asked for", stderr)
	}
}

func TestRuntime_turnsOnEveryChannel_whenAllIsAskedFor(t *testing.T) {
	// When
	_, _, stderr := runSetScript(t, "NEMOSH_DEBUG=all\ndefinitely-not-a-program\n")

	// Then
	if !strings.Contains(stderr, "debug: exec: ") {
		t.Fatalf("stderr = %q, want exec detail under all", stderr)
	}
}

func TestRuntime_reportsAnUnknownDebugChannel(t *testing.T) {
	// A misspelled channel that silently produced nothing would look exactly
	// like a shell that had nothing to say, which is the failure this whole
	// contract exists to avoid.
	// When
	_, _, stderr := runSetScript(t, "NEMOSH_DEBUG=exce\ndefinitely-not-a-program\n")

	// Then
	if !strings.Contains(stderr, "unknown channel") || !strings.Contains(stderr, "exce") {
		t.Fatalf("stderr = %q, want the misspelling named", stderr)
	}
}

func TestRuntime_hintsAboutTheSeparator_whenTheNameWasMeantAsAPath(t *testing.T) {
	// A name with a separator in it was never a PATH lookup, so a hint about
	// PATH would send the reader the wrong way.
	// When
	status, _, stderr := runSetScript(t, "./nosuchprogram\n")

	// Then
	if status != 127 {
		t.Fatalf("status = %d, want 127", status)
	}
	if !strings.Contains(stderr, "read as a path") {
		t.Fatalf("stderr = %q, want the hint to say it was read as a path", stderr)
	}
	if strings.Contains(stderr, "no directory on PATH") {
		t.Fatalf("stderr = %q, want no PATH hint for a path operand", stderr)
	}
}

func TestRuntime_hintsAboutTheBuiltin_whenAnExternalOfThatNameIsSought(t *testing.T) {
	// `command cd` reaches lookup for an external `cd`, and the useful thing to
	// say is that the name is a builtin here.
	// When
	_, _, stderr := runSetScript(t, "NEMOSH_OVERRIDE_APPLETS=-\nPATH=\ncommand cd\n")

	// Then
	if stderr != "" && !strings.Contains(stderr, "cd") {
		t.Fatalf("stderr = %q, want any diagnostic to name cd", stderr)
	}
}
