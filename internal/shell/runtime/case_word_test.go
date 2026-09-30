package runtime_test

import "testing"

// A case word and its patterns get every expansion but field splitting (POSIX 2.9.4.3),
// and a pattern's quoted characters are literal. busybox-w32 and bash 5.3 give each of
// these lines; nemosh split the word and the patterns to their first field, and read a
// quoted `*` as a pattern.
func TestCase_wordAndPatternsAsBothReferencesExpandThem(t *testing.T) {
	for _, test := range []struct{ name, script, want string }{
		{name: "a word with a blank", script: "x='a b'\ncase $x in 'a b') echo y;; *) echo n;; esac\n", want: "y\n"},
		{name: "a word with a newline", script: "x='one\ntwo'\ncase $x in one) echo first;; *two) echo whole;; esac\n", want: "whole\n"},
		{name: "a pattern with a blank", script: "p='a b'\ncase 'a b' in $p) echo y;; *) echo n;; esac\n", want: "y\n"},
		{name: "$@ as the word", script: "set -- 'x y' z\ncase $@ in 'x y z') echo y;; *) echo n;; esac\n", want: "y\n"},
		{name: "a quoted star", script: "case x in \"*\") echo n;; *) echo y;; esac\n", want: "y\n"},
		{name: "a quoted expansion", script: "p='*'\ncase x in \"$p\") echo n;; *) echo y;; esac\n", want: "y\n"},
		{name: "an unquoted expansion is a pattern", script: "p='*'\ncase x in $p) echo y;; esac\n", want: "y\n"},
		{name: "part quoted", script: "case ab in a\"*\") echo n;; a*) echo y;; esac\ncase 'a*' in a\"*\") echo y;; esac\n", want: "y\ny\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, stdout, stderr := runSetScript(t, test.script)
			if status != 0 || stdout != test.want {
				t.Fatalf("status %d stdout %q stderr %q, want %q", status, stdout, stderr, test.want)
			}
		})
	}
}

// The word after `case` is its subject, whatever it says, and that includes the word in:
// busybox and bash run a case on in. nemosh cut the header at the subject and refused the
// script, "case: expected: case word in".
func TestCase_theSubjectMayBeTheWordIn(t *testing.T) {
	for _, test := range []struct{ name, script, want string }{
		{name: "on one line", script: "case in in in) echo in;; esac\n", want: "in\n"},
		{name: "in a subshell", script: "(case in in in) echo sub;; esac)\n", want: "sub\n"},
		{name: "its in on the next line", script: "case in\nin\nin) echo next;; esac\n", want: "next\n"},
		{name: "a pattern that says in", script: "case x in in) echo no;; x) echo x;; esac\n", want: "x\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, stdout, stderr := runSetScript(t, test.script)
			if status != 0 || stdout != test.want {
				t.Fatalf("status %d stdout %q stderr %q, want %q", status, stdout, stderr, test.want)
			}
		})
	}
}
