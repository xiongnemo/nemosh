package applets_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// runAppletWithInput is runApplet with something on stdin, which grep needs:
// with no operand it reads the stream, and that is the shape an alias in a
// pipeline actually takes.
func runAppletWithInput(t *testing.T, input, name string, args ...string) (string, string, error) {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup(name)
	if !ok {
		t.Fatalf("expected %s applet to be registered", name)
	}
	var stdout, stderr bytes.Buffer
	err := applet.Run(context.Background(), args, strings.NewReader(input), &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

// `alias grep='grep --color=auto'` is in everybody's rc file, copied from a
// GNU-based system, and this shell sources `$ENV` precisely so a machine already
// configured for busybox needs no changes. Refusing the alias breaks every
// interactive `grep` on such a machine while `grep` from a script keeps working,
// which is a confusing way to fail.
//
// busybox accepts the option and ignores it: its table maps --color to a
// pseudo-flag with a NULL sink (findutils/grep.c:728) and nothing reads it. No
// colour is produced there, so none is produced here.
func TestGrep_acceptsTheColorOption_thatEveryAliasCarries(t *testing.T) {
	for _, args := range [][]string{
		{"--color=auto", "beta"},
		{"--color=always", "beta"},
		{"--color=never", "beta"},
		{"--color", "beta"},
		{"--color=auto", "-i", "BETA"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			// When
			stdout, _, err := runAppletWithInput(t, "alpha\nbeta xml\n", "grep", args...)

			// Then
			if err != nil {
				t.Fatalf("grep %v = %v, want it accepted", args, err)
			}
			if stdout != "beta xml\n" {
				t.Fatalf("stdout = %q, want the matching line and no escapes", stdout)
			}
		})
	}
}

// The diagnostic has to name what was typed. Reading a long option letter by
// letter reported the `-` it began with, so `--color=auto` came back as
// `unsupported grep option: --`, which names nothing and sent the reader looking
// for a `--` they never wrote.
func TestGrep_namesTheOptionItRefuses(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"--nonsense", "x"}, want: "unsupported grep option: --nonsense"},
		{args: []string{"--colour=auto", "x"}, want: "unsupported grep option: --colour=auto"},
		{args: []string{"-z", "x"}, want: "unsupported grep option: -z"},
		{args: []string{"--color=purple", "x"}, want: "unsupported --color value: purple"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			// When
			_, _, err := runAppletWithInput(t, "", "grep", test.args...)

			// Then
			if err == nil {
				t.Fatalf("grep %v succeeded, want a refusal", test.args)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

// The same defect lived in two more option parsers. None of them has a long
// option to accept, but naming the `-` a long option begins with instead of the
// option itself is what made this baffling in the first place.
func TestApplets_nameTheLongOptionTheyRefuse(t *testing.T) {
	for _, test := range []struct {
		applet string
		want   string
	}{
		{applet: "sort", want: "sort: unrecognized option --nonsense"},
		{applet: "uniq", want: "uniq: unrecognized option --nonsense"},
	} {
		t.Run(test.applet, func(t *testing.T) {
			// When
			_, stderr, err := runAppletWithInput(t, "x\n", test.applet, "--nonsense")

			// Then: cut, sort and uniq print their own diagnostics rather than
			// returning them, so the refusal can arrive by either route.
			if err == nil {
				t.Fatalf("%s --nonsense succeeded, want a refusal", test.applet)
			}
			reported := stderr
			if err != nil {
				reported += err.Error()
			}
			if !strings.Contains(reported, test.want) {
				t.Fatalf("%s reported %q, want it to contain %q", test.applet, reported, test.want)
			}
		})
	}
}

// `--` ends the options, so a pattern that begins with a dash can be searched
// for at all.
func TestGrep_treatsDoubleDashAsEndOfOptions(t *testing.T) {
	// When
	stdout, _, err := runAppletWithInput(t, "a-v-b\nplain\n", "grep", "--", "-v-")

	// Then
	if err != nil {
		t.Fatalf("grep -- -v- = %v, want the pattern searched for", err)
	}
	if stdout != "a-v-b\n" {
		t.Fatalf("stdout = %q, want the line containing the dashed pattern", stdout)
	}
}
