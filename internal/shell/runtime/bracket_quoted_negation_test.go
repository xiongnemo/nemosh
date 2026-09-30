package runtime_test

import "testing"

// A quoted or escaped ! or ^ at the start of a bracket expression is a member of the set, and
// not its negation, as busybox and bash match it: `case '!' in [\!])` matches. The quoting was
// lost on the way into the pattern, the set read as "not ]", and it never closed. Unquoted,
// they negate as before. busybox's ash_test quoted_punct.
func TestRuntime_aQuotedBangInABracketIsAMember(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"case '!' in [\\!]) echo ok;; *) echo wrong;; esac\n", "ok\n"},
		{"case '!' in [\"!\"]) echo ok;; *) echo wrong;; esac\n", "ok\n"},
		{"case '!' in ['!']) echo ok;; *) echo wrong;; esac\n", "ok\n"},
		{"case '^' in [\\^]) echo ok;; *) echo wrong;; esac\n", "ok\n"},
		{"case a in [\\!]) echo wrong;; *) echo ok;; esac\n", "ok\n"},
		{"x='!'; case \"$x\" in [\\!]) echo ok;; *) echo wrong;; esac\n", "ok\n"},
		{"x='!a'; echo \"${x#[\"!\"]}\"\n", "a\n"},
		{"case b in [!a]) echo ok;; *) echo wrong;; esac\n", "ok\n"},
		{"case b in [^a]) echo ok;; *) echo wrong;; esac\n", "ok\n"},
		{"case '!' in [!!]) echo wrong;; *) echo ok;; esac\n", "ok\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}
