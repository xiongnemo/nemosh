package runtime_test

import "testing"

// The fi, done or esac that ends a compound is a complete command, and a reserved word may
// follow it with no separator, as busybox and bash read it: `fi do`, `fi fi`, `fi done`, `done
// then` and `esac do`. The word after it was the closer's argument, and the compound around
// it never closed. A closer that is only a word stays one. busybox's ash_test
// groups_and_keywords1.
func TestRuntime_aReservedWordMayFollowAClosingWord(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"while if echo foo; then echo bar; fi do echo baz; break; done\n", "foo\nbar\nbaz\n"},
		{"if if true; then true; fi then echo y; fi\n", "y\n"},
		{"while case x in x) true;; esac do echo c; break; done\n", "c\n"},
		{"if while false; do :; done then echo w; fi\n", "w\n"},
		{"until for i in 1; do false; done do echo f; break; done\n", "f\n"},
		{"if true; then if true; then echo nest; fi fi\n", "nest\n"},
		{"while true; do if true; then break; fi done; echo ok\n", "ok\n"},
		{"case x in x) if true; then echo ci; fi esac\n", "ci\n"},
		{"case x in x) case y in y) echo cc;; esac esac\n", "cc\n"},
		{"if true; then for i in 1; do echo fd; done fi\n", "fd\n"},
		{"echo fi do done\n", "fi do done\n"},
		{"for w in fi done; do echo $w; done\n", "fi\ndone\n"},
		{"case x in fi) echo pattern;; x) echo x;; esac\n", "x\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
	// A closer that ends nothing is still refused, as in both references.
	if stdout, status := runScriptCapturing("for i in 1; do echo i; done done\n"); stdout != "" || status == 0 {
		t.Errorf("got %q/%d, want the script refused, as busybox and bash refuse it", stdout, status)
	}
}
