package applets

import "testing"

// User-defined functions.
//
// The sixteen cases below were measured against gawk and busybox before any of this was
// written, and the two agree on every one of them -- including the awkward middle case
// where a name that is neither a scalar nor an array yet becomes the caller's array because
// the callee used it as one.

func TestAwkFunctions(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name    string
		program string
		input   string
		want    string
	}{
		{
			// Scalars by value: the caller's `x` does not move.
			name: "a scalar is copied", program: `function f(a){a=99} BEGIN{x=1;f(x);print x}`, want: "1\n",
		},
		{
			// Arrays by reference: `B` and `A` are two names for one array.
			name: "an array is shared", program: `function f(A){A["k"]=99} BEGIN{f(B);print B["k"]}`, want: "99\n",
		},
		{
			// The extra-parameter idiom: `b` is the function's own, and the global of the
			// same name is untouched.
			name: "extra parameters are locals", program: `function f(a,   b){b=7;return a+b} BEGIN{print f(1), "["b"]"}`, want: "8 []\n",
		},
		{
			// A parameter shadows its global even when the function never assigns it.
			name: "a parameter shadows from the start", program: `function f(a,   g){return "["g"]"} BEGIN{g="global";print f(1), g}`, want: "[] global\n",
		},
		{name: "recursion", program: `function f(n){return n<=1?1:n*f(n-1)} BEGIN{print f(5)}`, want: "120\n"},
		{name: "deep recursion", program: `function f(n){if(n==0)return 0;return f(n-1)} BEGIN{print f(2000)}`, want: "0\n"},
		{
			// A bare `return` and falling off the end both answer the uninitialised value,
			// which is both "" and 0.
			name: "return with no value", program: `function f(){return} BEGIN{x=f();print "["x"]", x+0}`, want: "[] 0\n",
		},
		{name: "falling off the end", program: `function f(){} BEGIN{print "["f()"]"}`, want: "[]\n"},
		{name: "called before it is defined", program: `BEGIN{print g()} function g(){return "late"}`, want: "late\n"},
		{name: "missing arguments are uninitialised", program: `function f(a,b){return a"-"b} BEGIN{print f(1)}`, want: "1-\n"},
		{
			// Globals are visible and writable, including the record and its fields.
			name: "globals stay reachable", program: `function f(){$0="x y";return NF} BEGIN{print f(), $1}`, want: "2 x\n",
		},
		{name: "a local used as an array", program: `function f(A,   k){for(k in A)return k} BEGIN{B[7]=1;print f(B)}`, want: "7\n"},
		{
			// By reference all the way: emptying the parameter empties what was passed.
			name: "delete reaches the caller", program: `function f(A){delete A} BEGIN{B[1]=1;f(B);print length(B)}`, want: "0\n",
		},
		{
			// The middle case: `x` was never mentioned, so the callee decides it is an
			// array and the caller ends up holding it.
			name: "an untyped name becomes the caller's array", program: `function f(a){a[1]=1} BEGIN{f(x);print length(x)}`, want: "1\n",
		},
		{name: "split into a parameter", program: `function f(A){return split("a b c",A)} BEGIN{print f(B), B[2]}`, want: "3 b\n"},
		{
			// A name is not a keyword: a function may be called `len`.
			name: "a function may shadow nothing", program: `function len(s){return length(s)} BEGIN{print len("abc")}`, want: "3\n",
		},
		{
			// Arguments are evaluated in the caller's scope, which matters once the caller
			// is itself a function with a parameter of the same name.
			name: "arguments come from the caller's scope",
			program: `function inner(v){return v*2} function outer(v){return inner(v)+1}` +
				` BEGIN{print outer(5)}`, want: "11\n",
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, stderr, status := runAwk(t, testcase.program, testcase.input)
			if got != testcase.want || stderr != "" || status != 0 {
				t.Fatalf("%s\n got %q stderr %q status %d\nwant %q", testcase.program, got, stderr, status, testcase.want)
			}
			checkAgainstReferences(t, testcase.program, testcase.input, got)
		})
	}
}
