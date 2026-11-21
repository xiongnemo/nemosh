package oilsspec_test

import (
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/testutil/oilsspec"
)

// NEMOSH_OILS_AS=sh runs the cases with nemosh under sh, busybox's name for its ash, held to
// what the files record of ash; without it nemosh is bash, which the headline is about.
// Each name's PATH holds the helpers and nemosh's own directory, as bash's always has.
func TestSubjectAs_namesTheShellNemoshIsRunAs(t *testing.T) {
	installed := oilsspec.Installed{
		Helpers: filepath.FromSlash("C:/w/helpers"),
		Nemosh:  filepath.FromSlash("C:/w/nemosh/bash.exe"),
		Sh:      filepath.FromSlash("C:/w/nemosh-sh/sh.exe"),
	}
	for _, test := range []struct{ as, program, name, label string }{
		{as: "", program: installed.Nemosh, name: "bash", label: "bash"},
		{as: "bash", program: installed.Nemosh, name: "bash", label: "bash"},
		{as: "sh", program: installed.Sh, name: "sh", label: "ash"},
	} {
		subject, err := subjectAs(test.as, installed)
		if err != nil {
			t.Errorf("as %q: %v", test.as, err)
			continue
		}
		if subject.Program != test.program || subject.Name != test.name || subject.Label != test.label {
			t.Errorf("as %q: %s named %s, held to %s; want %s named %s, held to %s",
				test.as, subject.Program, subject.Name, subject.Label, test.program, test.name, test.label)
		}
		if len(subject.Path) != 2 || subject.Path[0] != installed.Helpers || subject.Path[1] != filepath.Dir(test.program) {
			t.Errorf("as %q: PATH is %q", test.as, subject.Path)
		}
	}
	if _, err := subjectAs("dash", installed); err == nil {
		t.Error("as dash: no error, but nemosh has no such name")
	}
}
