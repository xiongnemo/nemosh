package runtime_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// **A `case` keeps its arms inside a group.**
//
// Reported as a one-liner somebody expected to work, and it is the ordinary way to write a
// function that switches on its argument:
//
//	a() { case a in a) echo bingo;; *) echo hmm;; esac; }
//
// It answered `syntax error: unexpected )`. Both references run it -- busybox ash and bash
// print `bingo`.
//
// Two separate faults, found by narrowing rather than by reading:
//
//   - A brace group's body has its `;` turned into newlines so the rest of the parser can
//     treat it as a script, and every unquoted `;` counted -- including both halves of a
//     `;;`. So `a) echo one;; b) echo two` became two lines and the second began with `b)`.
//     One arm survived by accident, because POSIX lets the *last* arm omit its `;;`, which
//     is why this looked like it was about `*)` at first and was not.
//   - A subshell's extent is found by matching parentheses, and the `)` of a case pattern
//     closed it. The brace form already had a rule for this; the parenthesis form had the
//     same need and no rule.
//
// Both are about one construct being scanned by something that does not know it, which is
// why they are tested together.

func runOneLiner(t *testing.T, script string) (string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	status := rt.RunScript(context.Background(), script)
	if stderr.Len() != 0 {
		t.Logf("stderr: %s", strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), status
}

func TestCase_keepsItsArmsInsideAGroup(t *testing.T) {
	for _, testcase := range []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "the reported one-liner",
			script: "a() { case a in a) echo bingo;; *) echo hmm;; esac; }\na\n",
			want:   "bingo\n",
		},
		{
			name:   "the arm that is not first",
			script: "a() { case b in a) echo one;; b) echo two;; esac; }\na\n",
			want:   "two\n",
		},
		{
			name:   "three arms, middle taken",
			script: "a() { case b in a) echo one;; b) echo two;; c) echo three;; esac; }\na\n",
			want:   "two\n",
		},
		{
			name:   "a brace group rather than a function",
			script: "{ case a in a) echo one;; b) echo two;; esac; }\n",
			want:   "one\n",
		},
		{
			name:   "a subshell",
			script: "( case a in a) echo one;; *) echo hmm;; esac )\n",
			want:   "one\n",
		},
		{
			name:   "a subshell whose arm is not first",
			script: "( case z in a) echo one;; *) echo hmm;; esac )\n",
			want:   "hmm\n",
		},
		{
			name:   "nested: a case inside a case inside a function",
			script: "a() { case a in a) case b in b) echo deep;; esac;; *) echo hmm;; esac; }\na\n",
			want:   "deep\n",
		},
		{
			name:   "an arm running two commands",
			script: "a() { case a in a) echo one; echo two;; *) echo hmm;; esac; }\na\n",
			want:   "one\ntwo\n",
		},
		{
			name:   "the last arm may still omit its terminator",
			script: "a() { case b in a) echo one;; b) echo two; esac; }\na\n",
			want:   "two\n",
		},
		{
			name:   "a pattern list",
			script: "a() { case b in a|b) echo either;; *) echo hmm;; esac; }\na\n",
			want:   "either\n",
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			out, status := runOneLiner(t, testcase.script)
			if out != testcase.want || status != 0 {
				t.Errorf("stdout = %q, status = %d; want %q and 0", out, status, testcase.want)
			}
		})
	}
}

// TestCase_groupSeparatorsStillSeparate is the other direction, and the one a fix for the
// above could break: an ordinary `;` inside a group still ends a command, and a `;;` where
// no case is open is still wrong.
func TestCase_groupSeparatorsStillSeparate(t *testing.T) {
	out, status := runOneLiner(t, "{ echo one; echo two; }\n")
	if out != "one\ntwo\n" || status != 0 {
		t.Errorf("stdout = %q, status = %d; want two lines and 0", out, status)
	}
	if out, status := runOneLiner(t, "a() { echo one;; echo two; }\na\n"); status == 0 {
		t.Errorf("a `;;` with no case open was accepted: stdout = %q, status = %d", out, status)
	}
}
