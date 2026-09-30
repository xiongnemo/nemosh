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
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}
