package oilsspec_test

import (
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/testutil/oilsspec"
)

// A case is scored as test/sh_spec.py scores it: what a shell is held to is the case's
// default, key by key, unless a qualified line gives that shell its own value for the
// key; status is 0 when nothing says otherwise; and the result is the least of the
// assertions', so one mismatch fails the case and a matched N-I line makes it N-I.
func TestExpect_scoresARunAsOilsDoes(t *testing.T) {
	const spec = "#### case\ntrue\n" +
		"## stdout: x\n" +
		"## N-I dash status: 2\n" +
		"## BUG mksh stdout-json: \"\"\n" +
		"## OK zsh STDOUT:\ny\n## END\n" +
		"## OK zsh status: 1\n"
	tests := []struct {
		name  string
		shell string
		run   oilsspec.Output
		want  oilsspec.Result
	}{
		{"the default, met", "bash", oilsspec.Output{Stdout: "x\n"}, oilsspec.Pass},
		{"stderr, when nothing asserts it, is not looked at", "bash", oilsspec.Output{Stdout: "x\n", Stderr: "noise\n"}, oilsspec.Pass},
		{"status 0 is expected when nothing says otherwise", "bash", oilsspec.Output{Stdout: "x\n", Status: 1}, oilsspec.Fail},
		{"stdout differs", "bash", oilsspec.Output{Stdout: "y\n"}, oilsspec.Fail},
		{"a label names its shell by prefix", "bash-5.3", oilsspec.Output{Stdout: "x\n"}, oilsspec.Pass},
		{"a qualified status, with the default stdout still held", "dash", oilsspec.Output{Stdout: "x\n", Status: 2}, oilsspec.NotImplemented},
		{"the default stdout is still held beside a qualified status", "dash", oilsspec.Output{Stdout: "y\n", Status: 2}, oilsspec.Fail},
		{"a qualified status replaces the default one", "dash", oilsspec.Output{Stdout: "x\n"}, oilsspec.Fail},
		{"stdout-json is decoded", "mksh", oilsspec.Output{}, oilsspec.Bug},
		{"a qualified value replaces the default", "mksh", oilsspec.Output{Stdout: "x\n"}, oilsspec.Fail},
		{"qualified output and status together", "zsh", oilsspec.Output{Stdout: "y\n", Status: 1}, oilsspec.OK},
		{"a Python traceback on stderr fails any case", "bash", oilsspec.Output{Stdout: "x\n", Stderr: "Traceback (most recent call last):\n"}, oilsspec.Fail},
	}
	c := parseOne(t, spec)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected, err := c.Expect(test.shell)
			if err != nil {
				t.Fatal(err)
			}
			if got, _ := expected.Check(test.run); got != test.want {
				t.Errorf("%s run %+v scored %v, want %v", test.shell, test.run, got, test.want)
			}
		})
	}
}

// Both a plain and a -json value for one key are both held, as sh_spec.py makes an
// assertion of each, and a mismatch says what was expected and what came.
func TestExpect_holdsEveryValueGivenAndSaysWhatDiffered(t *testing.T) {
	c := parseOne(t, "#### case\ntrue\n## stdout: x\n## stdout-json: \"y\\n\"\n")
	expected, err := c.Expect("bash")
	if err != nil {
		t.Fatal(err)
	}

	got, messages := expected.Check(oilsspec.Output{Stdout: "x\n"})

	if got != oilsspec.Fail || len(messages) != 1 {
		t.Fatalf("scored %v with %q, want one failure", got, messages)
	}
	if want := `stdout: expected "y\n", got "x\n"`; messages[0] != want {
		t.Errorf("message %q, want %q", messages[0], want)
	}
}

func TestExpect_refusesAValueItCannotRead(t *testing.T) {
	for _, spec := range []string{
		"#### case\ntrue\n## status: two\n",
		"#### case\ntrue\n## N-I dash status: \n",
		"#### case\ntrue\n## stdout-json: 1\n",
		"#### case\ntrue\n## BUG dash stderr-json: \"\n",
	} {
		c := parseOne(t, spec)
		if _, err := c.Expect("dash"); err == nil {
			t.Errorf("%q: Expect succeeded, want an error", spec)
		}
	}
}

// A case is known by its description, and the second of two cases a file describes
// alike by the description and " #2", so each has a name a baseline can key on.
func TestParse_namesEachCaseUniquely(t *testing.T) {
	spec, err := oilsspec.Parse("#### same\ntrue\n#### other\ntrue\n#### same\ntrue\n#### same\ntrue\n")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, c := range spec.Cases {
		ids = append(ids, c.ID)
	}
	if got, want := strings.Join(ids, "|"), "same|other|same #2|same #3"; got != want {
		t.Errorf("ids %q, want %q", got, want)
	}
}

func parseOne(t *testing.T, input string) oilsspec.Case {
	t.Helper()
	spec, err := oilsspec.Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Cases) != 1 {
		t.Fatalf("parsed %d cases, want 1", len(spec.Cases))
	}
	return spec.Cases[0]
}
