package runtime_test

import "testing"

// The word after `case` is its subject, and a word where a pattern goes is a pattern, whatever
// they say, as busybox and bash read them. The bracket scans took the subject esac for the
// case's close, so in a subshell, a brace group and a command substitution the pattern's `)`
// was a bracket: "unexpected ;;". busybox's ash_test case1.
func TestRuntime_aCaseSubjectIsAWord(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"(case esac in \"esac\") echo sub;; esac)\n", "sub\n"},
		{"{ case esac in \"esac\") echo group;; esac; }\n", "group\n"},
		{"x=$(case esac in \"esac\") echo subst;; esac); echo $x\n", "subst\n"},
		{"f() ( case esac in \"esac\") echo body;; esac ); f\n", "body\n"},
		{"(case case in case) echo pattern;; esac)\n", "pattern\n"},
		{"(case x in a) ;; case) echo no;; x) echo later;; esac)\n", "later\n"},
		{"(case x in x) case y in y) echo inner;; esac;; esac)\n", "inner\n"},
		{"(case \"x\" in x) echo quoted;; esac)\n", "quoted\n"},
		{"(case \\x in x) echo escaped;; esac)\n", "escaped\n"},
		// Only the case's own `in` begins its patterns. A loop's in an arm, or an argument's,
		// put the `case` after it where a pattern goes, so that case counted for nothing and its
		// esac closed the outer one; git-completion.bash has a function written so, and was
		// "missing }".
		{"(case y in a) for c in 1; do case $c in esac; done;; *) echo loop;; esac)\n", "loop\n"},
		{"f() {\ncase y in\na)\n\tfor c in 1; do\n\t\tcase $c in\n\t\tesac\n\tdone\n\t;;\n*)\n\tfor c in a; do\n\t\tcase $c in\n\t\ta|b) echo function;;\n\t\tesac\n\tdone\nesac\n}\nf\n", "function\n"},
		{"f() { case y in a) echo in; case b in b) ;; esac;; *) echo argument | cat;; esac; }; f\n", "argument\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}
