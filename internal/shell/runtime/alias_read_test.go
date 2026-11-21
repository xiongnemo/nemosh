package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode an alias is substituted as the line is read, before its structure is decided,
// as bash and busybox both read one: an alias for `{`, `(`, a reserved word or half a loop is
// syntax, which nemosh and sh refuse on purpose. Ordinary aliases go on
// working: a trailing blank makes the next word one too, an alias is not substituted again in
// its own text, a line's own alias is not in force on it, a function's body is read with the
// aliases of its definition, and quotes keep a word from being one. Each answer is Oils'
// alias.test.sh's for bash or Git for Windows' bash 5.3's, measured 2026-11-22.
func TestAliases_bashModeReadsThemBeforeTheStructure(t *testing.T) {
	for _, test := range []struct{ name, script, stdout string }{
		{name: "an alias for a left brace", script: "alias LEFT='{'\nLEFT echo one; echo two; }\n", stdout: "one\ntwo\n"},
		{name: "an alias for a left paren", script: "alias LEFT='('\nLEFT echo one; echo two )\n", stdout: "one\ntwo\n"},
		{name: "a loop split across an alias", script: "alias e_='for i in 1 2 3; do echo '\ne_ $i; done\n", stdout: "1\n2\n3\n"},
		{name: "a loop split across chained aliases",
			script: "alias FOR1='for '\nalias FOR2='FOR1 '\nalias eye1='i '\nalias eye2='eye1 '\nalias IN='in '\nalias onetwo='$one \"2\" '\none=1\nFOR2 eye2 IN onetwo 3; do echo $i; done\n",
			stdout: "1\n2\n3\n"},
		{name: "an empty alias before an arithmetic command", script: "alias a=\na (( var = 0 ))\necho done\n", stdout: "done\n"},
		{name: "an alias for a keyword", script: "alias IF=if\nIF true; then echo yes; fi\n", stdout: "yes\n"},
		{name: "an ordinary alias", script: "alias ll='echo ll'\nll x\n", stdout: "ll x\n"},
		{name: "not again in its own text", script: "alias echo='echo pre'\necho x\n", stdout: "pre x\n"},
		{name: "a trailing blank", script: "alias hi='echo hello world '\nalias punct='!!!'\nhi punct\n", stdout: "hello world !!!\n"},
		{name: "not on its own line", script: "alias e2='echo e2'; e2 same 2>/dev/null\ne2 next\n", stdout: "e2 next\n"},
		{name: "not in quotes", script: "alias ll='echo ll'\n'll' 2>/dev/null; echo st=$?\n", stdout: "st=127\n"},
		{name: "in a command substitution", script: "alias ll='echo ll'\nx=$(ll y); echo \"$x\"\n", stdout: "ll y\n"},
		{name: "after a separator and in a group", script: "alias ll='echo ll'\ntrue && ll a; { ll b; }\n", stdout: "ll a\nll b\n"},
		{name: "a body read before the alias", script: "f() { ll; }\nalias ll='echo ll'\nf 2>/dev/null; echo st=$?\n", stdout: "st=127\n"},
		{name: "a body read after it", script: "alias ll='echo ll'\nf() { ll; }\nunalias ll\nf\n", stdout: "ll\n"},
		{name: "eval's text", script: "alias ll='echo ll'\neval 'll z'\n", stdout: "ll z\n"},
		{name: "not a case pattern nor a for list", script: "alias ll='echo ll'\ncase ll in ll) echo arm;; esac\nfor w in ll; do echo $w; done\n", stdout: "arm\nll\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
			rt.EnterBashMode()
			rt.RunScript(context.Background(), "shopt -s expand_aliases\n"+test.script)
			if stdout.String() != test.stdout {
				t.Fatalf("stdout %q, stderr %q; want %q", stdout.String(), stderr.String(), test.stdout)
			}
		})
	}
}
