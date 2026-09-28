package applets_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// echo -e, printf's format and %b read \uHHHH and \UHHHHHHHH as the character they name, in
// UTF-8, as bash reads them; busybox-w32 has none of them. They were printed back as written.
// With no hex digit after it the sequence is left as it was.
//
// A backslash is written ~ in the table and put back by escaped, so each case reads as it is
// typed at a prompt.
func TestEscapes_readUnicodeCharacters(t *testing.T) {
	escaped := func(text string) string { return strings.ReplaceAll(text, "~", `\`) }
	for index, test := range []struct {
		applet string
		args   []string
		want   string
	}{
		{applet: "printf", args: []string{"~u03bc ~U000003bc|~u00e9x~n"}, want: "μ μ|éx\n"},
		{applet: "printf", args: []string{"%b~n", "~u2500~U0001F600"}, want: "─😀\n"},
		{applet: "echo", args: []string{"-e", "~u3bc|~U3BC"}, want: "μ|μ\n"},
		{applet: "printf", args: []string{"~u ~ug ~U~n"}, want: escaped("~u ~ug ~U") + "\n"},
		{applet: "printf", args: []string{"~u12345~n"}, want: "ሴ5\n"},
	} {
		applet, ok := applets.DefaultRegistry.Lookup(test.applet)
		if !ok {
			t.Fatalf("%s is not registered", test.applet)
		}
		args := make([]string, len(test.args))
		for position, arg := range test.args {
			args[position] = escaped(arg)
		}
		var stdout bytes.Buffer
		if err := applet.Run(context.Background(), args, &bytes.Buffer{}, &stdout, &bytes.Buffer{}); err != nil {
			t.Fatalf("%d: %s %q: %v", index, test.applet, args, err)
		}
		if stdout.String() != test.want {
			t.Errorf("%d: %s %q printed %q, want %q", index, test.applet, args, stdout.String(), test.want)
		}
	}
}
