package runtime_test

import (
	"strings"
	"testing"
)

// A write that reads its element first evaluates the subscript once, as bash does: `+=` on an
// element, and `++`, `--` and the compound operators in arithmetic. It was evaluated for the
// read and again for the write, so `a[i++]+=x` stepped i twice and appended element 0's value to
// element 1. A compound operator reads its target before its right side, and `=` evaluates the
// subscript after it, as there. busybox has no arrays; the answers are bash 5.3's.
func TestRuntime_subscriptIsEvaluatedOnce(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"a=(1 2 3); i=0; a[i++]+=x; echo \"$i ${a[*]}\"\n", "1 1x 2 3\n"},
		{"declare -i n=(5); k=0; n[k++]+=2; echo \"$k ${n[*]}\"\n", "1 7\n"},
		{"c=(1 2 3); j=0; declare c[j++]+=x; echo \"$j ${c[*]}\"\n", "1 1x 2 3\n"},
		{"b=(1 2 3); i=0; (( b[i++] += 10 )); echo \"$i ${b[*]}\"\n", "1 11 2 3\n"},
		{"b=(5 6 7); i=0; (( b[i++]++ )); echo \"$i ${b[*]}\"\n", "1 6 6 7\n"},
		{"b=(5 6 7); i=0; (( ++b[i++] )); echo \"$i ${b[*]}\"\n", "1 6 6 7\n"},
		{"i=0; (( a[i++] += i )); echo \"$i ${a[*]}\"\n", "1 1\n"},
		{"i=0; (( a[i++] = i )); echo \"$i ${a[*]}\"\n", "1 0\n"},
		{"b=(5 6 7); i=1; (( b[i] = i++ )); echo \"$i ${b[*]}\"\n", "2 5 6 1\n"},
		{"x=1; (( x += x++ )); echo $x\n", "2\n"},
		{"x=1; (( x += (x=5) )); echo $x\n", "6\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}

// A read before the front of an array is empty and says so, `a: bad array subscript`, as bash
// says it; it was quiet. An arithmetic write there is refused and the expression goes on with its
// value, as there: it ended the script as a readonly variable, which it was not.
func TestRuntime_readBeforeTheFrontSaysSo(t *testing.T) {
	for _, test := range []struct {
		script, want string
		complaints   int
	}{
		{"a=(x); echo \"[${a[-2]}] [$((a[-2]))]\"\n", "[] [0]\n", 2},
		{"a=(x); [[ -v a[-2] ]]; echo $?\n", "1\n", 1},
		{"s=str; echo \"[${s[-2]}] [${s[-1]}] [${s[0]}]\"\n", "[] [] [str]\n", 2},
		{"a=(1); echo \"[$(( a[-2] += 1 ))]\"; echo next\n", "[1]\nnext\n", 2},
		{"a=(1); (( a[-2] = 5 )); echo \"st=$?\"\n", "st=0\n", 1},
		{"a=(1); a[-2]+=x; echo \"st=$?\"\necho next\n", "next\n", 1},
	} {
		t.Run(test.script, func(t *testing.T) {
			status, stdout, stderr := runSetScript(t, test.script)
			if stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
			if got := strings.Count(stderr, "bad array subscript"); got != test.complaints {
				t.Errorf("stderr %q says bad array subscript %d times, want %d", stderr, got, test.complaints)
			}
		})
	}
}
