package applets

import "testing"

// The built-in functions, each case checked against a literal answer *and* against gawk and
// busybox where they agree with it.
//
// The program text reaches the references through exec.Command rather than through a shell,
// so a backslash in a case here is the backslash awk sees. That matters more than usual:
// measuring these rules through Git Bash produced three different wrong answers before the
// fixtures were built in Go instead, which is the hazard AGENTS.md records.

func TestAwkStringBuiltins(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name     string
		program  string
		input    string
		want     string
		diverges []string
	}{
		{name: "substr takes a length", program: `BEGIN{print substr("hello",2,3)}`, want: "ell\n"},
		{name: "substr runs to the end", program: `BEGIN{print substr("hello",2)}`, want: "ello\n"},
		{
			// The fiddly one: the start moves to 1 and the length is *kept*, so this is
			// three characters and not two. Taking the characters between positions 0 and
			// 0+3-1 would answer "he".
			name: "substr clamps the start and keeps the length", program: `BEGIN{print substr("hello",0,3)}`, want: "hel\n",
		},
		{name: "substr with a negative start", program: `BEGIN{print substr("hello",-2,4)}`, want: "hell\n"},
		{name: "substr past the end", program: `BEGIN{print "["substr("hello",10)"]"}`, want: "[]\n"},
		{name: "substr with a length past the end", program: `BEGIN{print substr("hello",2,100)}`, want: "ello\n"},
		{name: "substr with a negative length", program: `BEGIN{print "["substr("hello",2,-1)"]"}`, want: "[]\n"},
		{
			// Truncated, not rounded: rounding would answer "el".
			name: "substr truncates its arguments", program: `BEGIN{print substr("hello",1.5,2.4)}`, want: "he\n",
		},
		{
			// 1e20 does not fit in an int64, and converting it would be undefined.
			// busybox overflows and answers nothing at all.
			name: "substr survives an enormous length", program: `BEGIN{print substr("hello",2,1e20)}`, want: "ello\n",
			diverges: []string{"busybox awk"},
		},
		{name: "index finds a substring", program: `BEGIN{print index("hello","ll")}`, want: "3\n"},
		{name: "index answers 0 when absent", program: `BEGIN{print index("hello","z")}`, want: "0\n"},
		{name: "length of a string", program: `BEGIN{print length("hello")}`, want: "5\n"},
		{name: "length with no argument is the record", program: `{print length}`, input: "abcd\n", want: "4\n"},
		{name: "length of a number goes through CONVFMT", program: `BEGIN{print length(12345)}`, want: "5\n"},
		{name: "length of an array counts its elements", program: `BEGIN{a[1];a[2];a[3];print length(a)}`, want: "3\n"},
		{name: "tolower and toupper", program: `BEGIN{print toupper("aBc"), tolower("aBc")}`, want: "ABC abc\n"},
		{name: "split on a regular expression", program: `BEGIN{n=split("a1b2c",A,/[0-9]/);print n,A[1],A[3]}`, want: "3 a c\n"},
		{
			// A single character is a literal, which is the FS rule: `/./` would answer 6.
			name: "split on a single character is literal", program: `BEGIN{n=split("a.b.c",A,".");print n,A[1]}`, want: "3 a\n",
		},
		{name: "split on a regex literal is not", program: `BEGIN{n=split("a.b.c",A,/./);print n"["A[1]"]"}`, want: "6[]\n"},
		{name: "split with no separator uses FS", program: `BEGIN{n=split("  a  b  ",A);print n"["A[1]"]["A[2]"]"}`, want: "2[a][b]\n"},
		{name: "split keeps empty fields", program: `BEGIN{n=split("a::b",A,":");print n"["A[2]"]"}`, want: "3[]\n"},
		{name: "split of nothing is nothing", program: `BEGIN{print split("",A)}`, want: "0\n"},
		{name: "split elements are strnums", program: `BEGIN{split("1 2",A);print (A[1]==1)}`, want: "1\n"},
		{name: "split replaces what was there", program: `BEGIN{A[9]="x";split("a",A);print length(A), A[9]""}`, want: "1 \n"},
		{name: "match records where it matched", program: `BEGIN{print match("hello",/l+/),RSTART,RLENGTH}`, want: "3 3 2\n"},
		{
			// RLENGTH is -1 rather than 0, which is what a program tests.
			name: "match records a failure", program: `BEGIN{print match("hello",/z/),RSTART,RLENGTH}`, want: "0 0 -1\n",
		},
		{name: "match takes a dynamic pattern", program: `BEGIN{print match("hello","l+"),RSTART,RLENGTH}`, want: "3 3 2\n"},
		{
			// Leftmost-longest, which is why regexp.CompilePOSIX is used: Go's default
			// engine would answer 1 1 1.
			name: "match is leftmost-longest", program: `BEGIN{print match("ab",/a|ab/),RSTART,RLENGTH}`, want: "1 1 2\n",
		},
		{name: "sub replaces the first match", program: `BEGIN{s="hello world";n=sub(/o/,"0",s);print n,s}`, want: "1 hell0 world\n"},
		{name: "gsub replaces every match", program: `BEGIN{s="hello world";n=gsub(/o/,"0",s);print n,s}`, want: "2 hell0 w0rld\n"},
		{name: "sub defaults to the record", program: `{sub(/two/,"2");print $0,NF}`, input: "one two\n", want: "one 2 2\n"},
		{name: "gsub rewrites a field and the record", program: `{gsub(/x/,"y",$2);print $0"|"NF}`, input: "p xqx r\n", want: "p yqy r|3\n"},
		{name: "gsub rewrites an array element", program: `BEGIN{A["k"]="axa";gsub(/a/,"b",A["k"]);print A["k"]}`, want: "bxb\n"},
		{name: "gsub takes a dynamic pattern", program: `BEGIN{s="ab";print gsub("b","X",s),s}`, want: "1 aX\n"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, stderr, status := runAwk(t, testcase.program, testcase.input)
			if got != testcase.want || stderr != "" || status != 0 {
				t.Fatalf("%s\n got %q stderr %q status %d\nwant %q", testcase.program, got, stderr, status, testcase.want)
			}
			checkAgainstReferences(t, testcase.program, testcase.input, got, testcase.diverges...)
		})
	}
}

// TestAwkBuiltinsCountRunes covers the one rule here that is a decision rather than a
// measurement.
//
// busybox counts bytes always; gawk counts bytes under `LC_ALL=C` and runes under a UTF-8
// locale, so the two only appeared to agree. Runes are chosen for the reason
// awk_builtin.go gives, so both references are declared as diverging -- which in CI, where
// gawk may well be in a UTF-8 locale and agree, costs nothing.
func TestAwkBuiltinsCountRunes(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name    string
		program string
		want    string
	}{
		{name: "length", program: `BEGIN{print length("héllo")}`, want: "5\n"},
		{name: "index", program: `BEGIN{print index("héllo","l")}`, want: "3\n"},
		{name: "substr does not cut a rune in half", program: `BEGIN{print substr("héllo",2,2)}`, want: "él\n"},
		{name: "split on an empty separator", program: `BEGIN{print split("héllo",A,""), A[2]}`, want: "5 é\n"},
		{name: "match reports rune offsets", program: `BEGIN{match("héllo",/l+/);print RSTART,RLENGTH}`, want: "3 2\n"},
		{name: "toupper knows more than ASCII", program: `BEGIN{print toupper("héllo")}`, want: "HÉLLO\n"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, _, _ := runAwk(t, testcase.program, "")
			if got != testcase.want {
				t.Fatalf("%s\n got %q\nwant %q", testcase.program, got, testcase.want)
			}
			checkAgainstReferences(t, testcase.program, "", got, "gawk", "busybox awk")
		})
	}
}

// TestAwkReplacementEscapes covers `&` in a sub or gsub replacement.
//
// The escapes were already decoded by the lexer, so a source `"[\\&]"` is the three
// characters `[\&]` by the time the replacement is expanded, and *that* is what makes it a
// literal ampersand. gawk alone turns a `\\` that no `&` follows into two backslashes;
// POSIX and busybox make it one.
func TestAwkReplacementEscapes(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name     string
		program  string
		want     string
		diverges []string
	}{
		{name: "ampersand is the matched text", program: `BEGIN{s="hello";gsub(/l/,"[&]",s);print s}`, want: "he[l][l]o\n"},
		{name: "escaped ampersand is a literal one", program: `BEGIN{s="hello";gsub(/l/,"[\\&]",s);print s}`, want: "he[&][&]o\n"},
		{name: "a backslash before the ampersand", program: `BEGIN{s="hello";gsub(/l/,"[\\\\&]",s);print s}`, want: `he[\l][\l]o` + "\n"},
		{name: "a lone backslash stands for itself", program: `BEGIN{s="hello";gsub(/l/,"\\",s);print s}`, want: `he\\o` + "\n"},
		{
			name: "a doubled backslash is one", program: `BEGIN{s="hello";gsub(/l/,"\\\\",s);print s}`,
			want: `he\\o` + "\n", diverges: []string{"gawk"},
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, _, _ := runAwk(t, testcase.program, "")
			if got != testcase.want {
				t.Fatalf("%s\n got %q\nwant %q", testcase.program, got, testcase.want)
			}
			checkAgainstReferences(t, testcase.program, "", got, testcase.diverges...)
		})
	}
}

// TestAwkGsubEmptyMatches covers the rule that decides how many times a pattern which can
// match nothing gets replaced.
//
// **An empty match abutting the previous one is skipped**, so `gsub(/a*/,"-","aaa")`
// replaces once. busybox counts the empty match at the end of the string as a second one
// and answers 2. Go's FindAllStringIndex already has gawk's rule, and using it also keeps
// `^` and `$` anchored to the whole string.
func TestAwkGsubEmptyMatches(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name     string
		program  string
		want     string
		diverges []string
	}{
		{
			name: "a pattern that swallows the string", program: `BEGIN{s="aaa";n=gsub(/a*/,"-",s);print n,s}`,
			want: "1 -\n", diverges: []string{"busybox awk"},
		},
		{
			name: "empty matches between the letters", program: `BEGIN{s="baaac";n=gsub(/a*/,"-",s);print n,s}`,
			want: "3 -b-c-\n", diverges: []string{"busybox awk"},
		},
		{
			name: "a pattern matching nothing at all", program: `BEGIN{s="abc";n=gsub(/b*/,"-",s);print n,s}`,
			want: "3 -a-c-\n", diverges: []string{"busybox awk"},
		},
		{name: "the empty pattern", program: `BEGIN{s="abc";n=gsub(//,"-",s);print n,s}`, want: "4 -a-b-c-\n"},
		{name: "an anchor at the end", program: `BEGIN{s="abc";n=gsub(/$/,"X",s);print n,s}`, want: "1 abcX\n"},
		{name: "an anchor at the start", program: `BEGIN{s="abc";n=gsub(/^/,"X",s);print n,s}`, want: "1 Xabc\n"},
		{name: "an empty subject", program: `BEGIN{s="";n=gsub(/x*/,"-",s);print n"["s"]"}`, want: "1[-]\n"},
		{name: "sub takes only the first", program: `BEGIN{s="aaa";n=sub(/a*/,"-",s);print n,s}`, want: "1 -\n"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, _, _ := runAwk(t, testcase.program, "")
			if got != testcase.want {
				t.Fatalf("%s\n got %q\nwant %q", testcase.program, got, testcase.want)
			}
			checkAgainstReferences(t, testcase.program, "", got, testcase.diverges...)
		})
	}
}
