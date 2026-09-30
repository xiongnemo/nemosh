package runtime_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// ${var@P} is bash's prompt expansion of a value: its escapes decoded with what each stands
// for quoted, then the whole expanded as double-quoted text. Each answer was measured in bash
// 5.3; busybox has no @P. It was refused as not implemented.
func TestParameter_promptTransform(t *testing.T) {
	symbol := "$"
	if applets.CurrentUserID() == 0 {
		symbol = "#"
	}
	tests := []struct{ name, script, stdout string }{
		{name: "decoded before it is expanded", script: "x='\\'; y=h; PS1='$x$y'\necho \"${PS1@P}\"\n", stdout: "\\h\n"},
		{name: "a parameter expanded after", script: "v=set; PS1='[$v]'\necho \"${PS1@P}\"\n", stdout: "[set]\n"},
		{name: "the symbol", script: "PS1='\\$'\necho \"${PS1@P}\"\n", stdout: symbol + "\n"},
		// A prompt's text is the inside of double quotes, so a # in it begins no comment, whoever
		// runs the test: root's `#` prompt came out empty, and a backquote after one with it.
		{name: "a hash", script: "PS1='# `echo hi` #'\necho \"${PS1@P}\"\n", stdout: "# hi #\n"},
		{name: "a backslash before a dollar", script: "PS1='\\\\$ \\\\\\\\$'\necho \"${PS1@P}\"\n", stdout: "$ \\$\n"},
		{name: "octal", script: "PS1='\\1004 [\\045] \\555'\necho \"${PS1@P}\"\n", stdout: "@4 [%] m\n"},
		{name: "a lone backslash", script: "PS1='\\'\necho \"${PS1@P}\"\n", stdout: "\\\n"},
		{name: "each element of a list", script: "set -- a b\necho ${@@P} ${*@P}\na=(x y)\necho ${a@P}\n", stdout: "a b a b\nx\n"},
		{name: "unset is nothing", script: "unset v\necho \"[${v@P}]\"\n", stdout: "[]\n"},
		{name: "the command number counts lines", script: "PS1='\\#'\na=${PS1@P}\nb=${PS1@P}\necho $((b - a))\n", stdout: "1\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 || stdout != test.stdout {
				t.Fatalf("got %q/%d, want %q/0; stderr = %q", stdout, status, test.stdout, stderr)
			}
		})
	}
}

// \w is the working directory with $HOME written ~, and the directory's name is quoted for
// the expansion that follows: one called `$foo` stays $foo.
func TestParameter_promptTransformWorkingDirectory(t *testing.T) {
	home := filepath.ToSlash(t.TempDir())
	script := "HOME='" + home + "'\ncd ~\nmkdir -p 'a/$foo'\nfoo=expanded\nPS1='\\w|\\W|$foo'\necho \"${PS1@P}\"\ncd 'a/$foo'\necho \"${PS1@P}\"\n"

	// When
	status, stdout, stderr := runSetScript(t, script)

	// Then
	if want := "~|~|expanded\n~/a/$foo|$foo|expanded\n"; status != 0 || stdout != want {
		t.Fatalf("got %q/%d, want %q/0; stderr = %q", stdout, status, want, stderr)
	}
	if strings.Contains(stdout, "expanded/") {
		t.Fatalf("a directory's name was expanded: %q", stdout)
	}
}
