package runtime_test

import "testing"

// Where a command begins, an assignment's subscript may hold blanks and operators, as bash reads
// it: `a[1 + 1]=x`, `a[5&3]=x`, `a[(1+2)*3]=9` and `a[ a[0]=1 ]=X` are assignments, two may
// stand side by side, and one may be a condition. As an argument the same text is words of its
// own, and in an arithmetic command and an array literal it is what it always was. Each was
// cut at its first blank or operator and run as a command. bash's answers, measured; busybox has
// no arrays.
func TestArraySubscript_holdsBlanksAndOperatorsWhereACommandBegins(t *testing.T) {
	script := "a[1 * 1]=x; a[ 1 + 2 ]=z; echo status=$?; printf '<%s>' \"${a[@]}\"; echo\n" +
		"b[b[0]=1]=X; b[ b[2]=3 ]=Y; declare -p b\n" +
		"c[5&3]=h; c[1|2]=w; c[(1+2)*3]=9; declare -p c\n" +
		"d=(0 1 2); e=(3 4 5); HOME=/home/t\n" +
		"d[0 + 1]=  e[2 + 0]=~/src; declare -p d e\n" +
		"printf '<%s>' f[2 + 0]=bar; echo\n" +
		"((g[1 + 1]=2)); declare -p g\n" +
		"arr=( h[1 + 1]=x ); declare -p arr\n" +
		"i=1; j[$i + 1]=k; declare -p j\n" +
		"if k[1 + 1]=v; then declare -p k; fi\n"
	want := "status=0\n<x><z>\n" +
		"declare -a b=([0]=\"1\" [1]=\"X\" [2]=\"3\" [3]=\"Y\")\n" +
		"declare -a c=([1]=\"h\" [3]=\"w\" [9]=\"9\")\n" +
		"declare -a d=([0]=\"0\" [1]=\"\" [2]=\"2\")\n" +
		"declare -a e=([0]=\"3\" [1]=\"4\" [2]=\"/home/t/src\")\n" +
		"<f[2><+><0]=bar>\n" +
		"declare -a g=([2]=\"2\")\n" +
		"declare -a arr=([0]=\"h[1\" [1]=\"+\" [2]=\"1]=x\")\n" +
		"declare -a j=([2]=\"k\")\n" +
		"declare -a k=([2]=\"v\")\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as bash answers", stdout, status, want)
	}
}
