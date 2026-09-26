package runtime_test

import (
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	shellruntime "github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// IFS is space, tab and newline when the shell starts, whatever the environment says, as
// both references have it. It was unset, so `OLD=$IFS; ...; IFS=$OLD` -- the ordinary way
// to change it for a while -- left it empty and word splitting off; and an inherited IFS=:
// was taken as given, so a caller could change how every script it ran split its words.
// An IFS that was inherited is passed on, as the references pass it on: with the default
// value. One that was not is not exported.
func TestRuntime_startsWithTheDefaultIFS(t *testing.T) {
	tests := []struct {
		name     string
		env      []string
		exported string
	}{
		{"with none inherited", nil, "not exported\n"},
		{"with one inherited", []string{"IFS=:"}, "IFS= \t\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			var stdout strings.Builder
			rt, err := shellruntime.NewRuntimeWithState(applets.DefaultRegistry,
				shellruntime.Streams{Stdout: &stdout},
				shellruntime.State{Cwd: shellruntime.WorkingDirectory(t.TempDir()), Env: shellruntime.NewEnvironment(test.env)})
			if err != nil {
				t.Fatal(err)
			}

			// When
			status := rt.RunScript(t.Context(), "echo ${#IFS}\n"+
				"OLD=$IFS; IFS=:; IFS=$OLD; set -- $(echo a b); echo $#\n"+
				"env | grep '^IFS=' || echo not exported\n")

			// Then
			if want := "3\n2\n" + test.exported; status != 0 || stdout.String() != want {
				t.Errorf("status %d, output %q; want 0, %q", status, stdout.String(), want)
			}
		})
	}
}
