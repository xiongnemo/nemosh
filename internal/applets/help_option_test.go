package applets_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func runAppletHelp(t *testing.T, name string, args ...string) (string, error) {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup(name)
	if !ok {
		t.Fatalf("%s is not registered", name)
	}
	var stdout, stderr bytes.Buffer
	err := applet.Run(context.Background(), args, strings.NewReader(""), &stdout, &stderr)
	if stderr.Len() > 0 {
		t.Logf("%s stderr: %s", name, stderr.String())
	}
	return stdout.String(), err
}

// Every applet answers --help. Before this each of them rejected it in its own
// words -- `ls: unsupported ls option: --help`, `du: invalid option -- '-'` -- which
// is the first thing anyone types.
func TestHelpOption_isAnsweredByEveryApplet(t *testing.T) {
	dataApplets := map[string]bool{"echo": true, "test": true, "[": true, "true": true, "false": true}
	for _, name := range applets.DefaultRegistry.Names() {
		if dataApplets[name] {
			continue
		}
		t.Run(name, func(t *testing.T) {
			// When
			stdout, err := runAppletHelp(t, name, "--help")

			// Then
			if err != nil {
				t.Fatalf("%s --help failed: %v", name, err)
			}
			if !strings.HasPrefix(stdout, "Usage: "+name) {
				t.Fatalf("%s --help printed %q, want a synopsis beginning `Usage: %s`",
					name, firstLine(stdout), name)
			}
			// A synopsis with no sentence under it is a worse answer than none: it
			// says how to type the command without saying what it is for.
			if len(strings.Split(strings.TrimSpace(stdout), "\n")) < 3 {
				t.Fatalf("%s --help is only %q, with no summary", name, stdout)
			}
		})
	}
}

// The exclusions, measured against busybox-w32: for these an argument is data, so
// `--help` has to reach the applet.
func TestHelpOption_isDataForTheAppletsThatTakeArgumentsAsData(t *testing.T) {
	t.Run("echo prints it", func(t *testing.T) {
		// When
		stdout, err := runAppletHelp(t, "echo", "--help")

		// Then
		if err != nil {
			t.Fatalf("echo --help failed: %v", err)
		}
		if stdout != "--help\n" {
			t.Fatalf("echo --help printed %q, want it treated as data", stdout)
		}
	})
	t.Run("test evaluates it", func(t *testing.T) {
		// When -- a non-empty string is true, so this must succeed and print nothing.
		stdout, err := runAppletHelp(t, "test", "--help")

		// Then
		if err != nil {
			t.Fatalf("test --help = %v, want success: the string --help is non-empty", err)
		}
		if stdout != "" {
			t.Fatalf("test --help printed %q, want nothing", stdout)
		}
	})
	t.Run("true ignores it", func(t *testing.T) {
		// When
		stdout, err := runAppletHelp(t, "true", "--help")

		// Then
		if err != nil || stdout != "" {
			t.Fatalf("true --help = (%q, %v), want nothing and success", stdout, err)
		}
	})
}

// `--` ends the options, so what follows is an operand even when it is spelled like
// a request for help. `grep -- --help file` is a search for that string.
func TestHelpOption_isAnOperandAfterADoubleDash(t *testing.T) {
	// When
	stdout, _ := runAppletHelp(t, "grep", "--", "--help")

	// Then
	if strings.HasPrefix(stdout, "Usage:") {
		t.Fatalf("grep -- --help printed usage; after -- it is the pattern to search for")
	}
}

// A mistyped option belongs to the applet's own parser, which can name it. Swallowing
// it here would answer a question nobody asked.
func TestHelpOption_doesNotMatchAPrefix(t *testing.T) {
	// When
	stdout, err := runAppletHelp(t, "ls", "--help-me")

	// Then
	if strings.HasPrefix(stdout, "Usage:") {
		t.Fatalf("ls --help-me printed usage, want its own diagnostic")
	}
	if err == nil {
		t.Fatal("ls --help-me should fail: it is not an option")
	}
}

// The usage text has to answer the question it exists for -- which options this
// build takes -- and that is what the capability matrix knows.
func TestHelpOption_namesTheOptionsTheAppletActuallyTakes(t *testing.T) {
	// When
	stdout, err := runAppletHelp(t, "grep", "--help")

	// Then
	if err != nil {
		t.Fatalf("grep --help: %v", err)
	}
	for _, want := range []string{"-i", "-v", "-r", "-m COUNT", "--color", "match without regard to case"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("grep --help does not mention %q:\n%s", want, stdout)
		}
	}
	// And it must not claim one it does not take: grep here has no -P.
	if strings.Contains(stdout, "-P") {
		t.Fatalf("grep --help mentions -P, which this build does not take:\n%s", stdout)
	}
}

// Usage goes to stdout and succeeds, so `ls --help | head` works. Only a *rejected*
// option is an error.
func TestHelpOption_goesToStdoutAndSucceeds(t *testing.T) {
	// Given
	applet, _ := applets.DefaultRegistry.Lookup("wc")
	var stdout, stderr bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"--help"}, strings.NewReader(""), &stdout, &stderr)

	// Then
	if err != nil {
		t.Fatalf("wc --help = %v, want success", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("wc --help wrote %q to stderr; usage is an answer, not a complaint", stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatal("wc --help wrote nothing to stdout")
	}
}

func firstLine(text string) string {
	if index := strings.IndexByte(text, '\n'); index >= 0 {
		return text[:index]
	}
	return text
}
