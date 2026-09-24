package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func runCoprocScript(t *testing.T, launcher, script string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	rt.env.Set("NEMOSH_JOBS", launcher)
	status := rt.RunScript(context.Background(), script)
	rt.CloseBatch(status)
	return status, stdout.String(), stderr.String()
}

// bash 5.3's answers, line for line, under both launchers; busybox has no coproc. The
// shell's ends are 63 and 60, closing the input ends the coprocess, and the wait that
// reaps it unsets its names.
func TestCoproc_givesBashsAnswers(t *testing.T) {
	const script = `coproc cat
echo "fds=${COPROC[0]} ${COPROC[1]} pid-set=${COPROC_PID:+yes}"
echo hello >&"${COPROC[1]}"
read line <&"${COPROC[0]}"
echo "got=$line"
exec {COPROC[1]}>&-
wait $COPROC_PID; echo "st=$?"
coproc UP { tr a-z A-Z; }
echo "named=${UP[0]} ${UP[1]} ${UP_PID:+pid}"
printf 'abc\n' >&"${UP[1]}"
exec {UP[1]}>&-
read up <&"${UP[0]}"; echo "up=$up"
wait $UP_PID; echo "st=$?"
echo "after=${UP[0]-unset} ${UP_PID-unset}"
`
	const want = "fds=63 60 pid-set=yes\ngot=hello\nst=0\nnamed=63 60 pid\nup=ABC\nst=0\nafter=unset unset\n"
	for _, launcher := range []string{"goroutine", "process"} {
		t.Run(launcher, func(t *testing.T) {
			status, stdout, stderr := runCoprocScript(t, launcher, script)
			if status != 0 || stdout != want {
				t.Fatalf("status %d stdout %q stderr %q\nwant %q", status, stdout, stderr, want)
			}
		})
	}
}

// Every form the parser has to take, each reaching the coprocess it names: a simple
// command, a named group across lines, one inside a compound, and a named loop. The
// line's own commands after the coprocess run in the shell, as in bash.
func TestCoproc_takesEachForm(t *testing.T) {
	for _, script := range []string{
		"coproc cat; echo hi >&\"${COPROC[1]}\"; exec {COPROC[1]}>&-; read l <&\"${COPROC[0]}\"; echo \"$l\"\n",
		"coproc W {\n  cat\n}\necho hi >&\"${W[1]}\"; exec {W[1]}>&-; read l <&\"${W[0]}\"; echo \"$l\"\n",
		"if true; then coproc { cat; }; fi\necho hi >&\"${COPROC[1]}\"; exec {COPROC[1]}>&-; read l <&\"${COPROC[0]}\"; echo \"$l\"\n",
		"coproc L while read l; do echo \"$l\"; done\necho hi >&\"${L[1]}\"; exec {L[1]}>&-; read l <&\"${L[0]}\"; echo \"$l\"\n",
	} {
		status, stdout, stderr := runCoprocScript(t, "goroutine", script+"wait\n")
		if status != 0 || stdout != "hi\n" {
			t.Errorf("%q = %d %q %q", script, status, stdout, stderr)
		}
	}
}

// A function holding a coprocess prints as one, and what it prints runs the same: this is
// the text a job process is sent.
func TestCoproc_printsAndReadsBack(t *testing.T) {
	script := "f() { coproc UP { tr a-z A-Z; }; echo x >&\"${UP[1]}\"; exec {UP[1]}>&-; read l <&\"${UP[0]}\"; echo \"$l\"; wait; }\n" +
		"text=$(declare -f f)\ncase \"$text\" in *'coproc UP {'*) echo printed;; esac\neval \"${text/f ()/g ()}\"\ng\n"
	status, stdout, stderr := runCoprocScript(t, "goroutine", script)
	if status != 0 || stdout != "printed\nX\n" || strings.Contains(stderr, "not implemented") {
		t.Fatalf("status %d stdout %q stderr %q", status, stdout, stderr)
	}
}
