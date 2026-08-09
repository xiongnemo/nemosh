package runtime_test

import (
	"strings"
	"testing"
)

// `export` with no operands lists what is exported, and so does `export -p`.
// POSIX specifies the -p form; busybox lists for both, and a shell that prints
// nothing gives a user no way to see the environment at all. Reported as
// nemosh issue #10.
//
// The format is `export NAME='value'`, quoted so the output can be fed back to
// a shell -- which is what POSIX means by "in a form that can be reused as
// input".
func TestExport_listsExportedVariables(t *testing.T) {
	for _, script := range []string{"export\n", "export -p\n"} {
		t.Run(strings.TrimSpace(script), func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, "FOO=bar\nexport FOO\n"+script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
			}
			if !strings.Contains(stdout, "export FOO='bar'\n") {
				t.Fatalf("stdout = %q, want it to contain %q", stdout, "export FOO='bar'\n")
			}
		})
	}
}

// A variable that was never exported is not listed, which is the difference
// between `export` and `set`.
func TestExport_omitsWhatIsNotExported(t *testing.T) {
	// When
	_, stdout, _ := runSetScript(t, "PLAIN=value\nexport\n")

	// Then
	if strings.Contains(stdout, "PLAIN") {
		t.Fatalf("stdout = %q, want it to omit a variable that was never exported", stdout)
	}
}

// Sorted, so two runs of the same script produce the same output and a diff
// between them means something.
func TestExport_listsInNameOrder(t *testing.T) {
	// When
	_, stdout, _ := runSetScript(t, "export ZED=1\nexport ALPHA=2\nexport MID=3\nexport\n")

	// Then
	var seen []string
	for _, line := range strings.Split(stdout, "\n") {
		for _, name := range []string{"ALPHA", "MID", "ZED"} {
			if strings.HasPrefix(line, "export "+name+"=") {
				seen = append(seen, name)
			}
		}
	}
	if strings.Join(seen, ",") != "ALPHA,MID,ZED" {
		t.Fatalf("listed %v, want them in name order", seen)
	}
}

// A value carrying a quote has to come back quoted so it can be reused.
func TestExport_quotesAValueForReuse(t *testing.T) {
	// When
	_, stdout, _ := runSetScript(t, "export Q=\"it's here\"\nexport\n")

	// Then
	if !strings.Contains(stdout, `export Q='it'\''s here'`) {
		t.Fatalf("stdout = %q, want the value quoted for reuse", stdout)
	}
}

// Assigning through export still works, and is not turned into a listing.
func TestExport_stillAssigns(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "export NAME=value\necho [$NAME]\n")

	// Then
	if status != 0 || stdout != "[value]\n" {
		t.Fatalf("status = %d, stdout = %q, stderr = %q", status, stdout, stderr)
	}
}
