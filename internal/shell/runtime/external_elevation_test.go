package runtime

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
)

// The words a reader gets back. `WinSAT` used to answer
// `fork/exec C:\Windows\system32\WinSAT.exe: The requested operation requires
// elevation.` -- "fork/exec" names no Windows API, and nothing in the sentence
// says whether the shell tried, gave up, or will never do it.
func TestElevationDiagnostic_saysWhoWillNotDoItAndWhatToDoInstead(t *testing.T) {
	// When
	diagnostic := elevationDiagnostic("WinSAT")

	// Then
	if strings.Contains(diagnostic.message, "fork/exec") {
		t.Fatalf("message = %q, want no Go-ism in it", diagnostic.message)
	}
	for _, fragment := range []string{"requires administrator", "does not elevate"} {
		if !strings.Contains(diagnostic.message, fragment) {
			t.Fatalf("message = %q, want it to contain %q", diagnostic.message, fragment)
		}
	}
	// The hint has to name the command, because a reader copying it out has one
	// less thing to get wrong, and has to name where the reasoning lives.
	for _, fragment := range []string{"WinSAT", "elevated shell", "support-matrix.md"} {
		if !strings.Contains(diagnostic.hint, fragment) {
			t.Fatalf("hint = %q, want it to contain %q", diagnostic.hint, fragment)
		}
	}
}

// Recognising the failure is the whole of it: everything else is wording. The
// error arrives wrapped in whatever os/exec puts around it, so this must match
// through the chain rather than by string.
func TestRequiresElevation_matchesThroughTheWrappedError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "the real shape, as os/exec returns it",
			err:  &os.PathError{Op: "fork/exec", Path: `C:\Windows\system32\WinSAT.exe`, Err: syscall.Errno(740)},
			want: elevationIsAWindowsIdea,
		},
		{
			name: "some other launch failure",
			err:  &os.PathError{Op: "fork/exec", Path: `C:\nope.exe`, Err: syscall.Errno(2)},
			want: false,
		},
		{
			name: "an error that is not a syscall at all",
			err:  errors.New("something else"),
			want: false,
		},
		{name: "nothing went wrong", err: nil, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := requiresElevation(test.err)

			// Then
			if got != test.want {
				t.Fatalf("requiresElevation(%v) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}
