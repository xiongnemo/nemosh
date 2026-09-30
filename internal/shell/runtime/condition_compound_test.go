package runtime_test

import "testing"

// A condition may begin with a compound after other words, `if ! if ...` and `if x && case
// ...`, and an elif's may begin with one, `elif if ...` and `elif while ...`, as busybox and
// bash read them. Each was "unexpected then", "unexpected do" or "unexpected ;;": the
// compound's then was taken for the condition's.
func TestRuntime_aConditionMayBeginWithACompound(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"if false; then echo a; elif if true; then true; fi; then echo b; fi\n", "b\n"},
		{"if false; then :; elif while false; do :; done; then echo w; fi\n", "w\n"},
		{"if false; then :; elif case x in x) true;; esac; then echo c; fi\n", "c\n"},
		{"if false; then :; elif for i in 1; do false; done; then :; else echo e; fi\n", "e\n"},
		{"if false; then :; elif ! if false; then :; else false; fi; then echo n; fi\n", "n\n"},
		{"if false; then :; elif true && if true; then true; fi; then echo a; fi\n", "a\n"},
		{"if false; then :; elif if false; then :; elif true; then :; fi; then echo d; fi\n", "d\n"},
		{"if ! if false; then :; else false; fi; then echo i; fi\n", "i\n"},
		{"if false || if true; then true; fi; then echo o; fi\n", "o\n"},
		{"if true | if true; then cat; fi; then echo p; fi\n", "p\n"},
		{"while ! if true; then false; fi; do echo x; break; done\n", "x\n"},
		{"if false; then :\nelif\n  if true; then true; fi\nthen echo m\nfi\n", "m\n"},
		{"if false\nthen :\nelif true &&\n  if true; then echo in; fi\nthen echo out\nfi\n", "in\nout\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox answers", stdout, status, test.want)
			}
		})
	}
}
