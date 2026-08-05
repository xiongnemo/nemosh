//go:build windows

package applets_test

import (
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestInvocationName_removesOnlyWindowsExecutableSuffix(t *testing.T) {
	tests := []struct {
		argv0 string
		want  string
	}{
		{argv0: `C:\bin\pwd.exe`, want: "pwd"},
		{argv0: `C:\bin\PWD.EXE`, want: "pwd"},
		{argv0: `C:\bin\pwd`, want: "pwd"},
		{argv0: `C:\bin\pwd.extra.exe`, want: "pwd.extra"},
		{argv0: `C:\bin\pwd.extra`, want: "pwd.extra"},
	}
	for _, test := range tests {
		t.Run(test.want+test.argv0, func(t *testing.T) {
			// When
			got := applets.InvocationName([]string{test.argv0})

			// Then
			if got != test.want {
				t.Fatalf("InvocationName(%q) = %q, want %q", test.argv0, got, test.want)
			}
		})
	}
}
