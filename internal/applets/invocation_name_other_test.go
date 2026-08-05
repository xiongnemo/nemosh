//go:build !windows

package applets_test

import (
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestInvocationName_preservesFinalExtensionsOnNonWindows(t *testing.T) {
	tests := []struct {
		argv0 string
		want  string
	}{
		{argv0: "/tmp/pwd", want: "pwd"},
		{argv0: "/tmp/pwd.extra", want: "pwd.extra"},
		{argv0: "/tmp/pwd.extra.bin", want: "pwd.extra.bin"},
		{argv0: "/tmp/cat.tool", want: "cat.tool"},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			// When
			got := applets.InvocationName([]string{test.argv0})

			// Then
			if got != test.want {
				t.Fatalf("InvocationName(%q) = %q, want %q", test.argv0, got, test.want)
			}
		})
	}
}
