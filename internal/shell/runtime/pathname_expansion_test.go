package runtime_test

import "testing"

// seedGlobTree writes a fixed set of names so the expansions below have
// something to match, then runs the script from that directory.
const seedGlobTree = "mkdir sub\n" +
	": > alpha.txt\n" +
	": > beta.txt\n" +
	": > gamma.md\n" +
	": > .hidden\n" +
	": > sub/inner.txt\n"

func TestRuntime_expandsPathnamePatterns(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "star with a suffix", script: "echo *.txt\n", want: "alpha.txt beta.txt\n"},
		{name: "star alone skips dotfiles", script: "echo *\n", want: "alpha.txt beta.txt gamma.md sub\n"},
		{name: "a leading dot must be spelled out", script: "echo .h*\n", want: ".hidden\n"},
		{name: "question matches one character", script: "echo gamm?.md\n", want: "gamma.md\n"},
		{name: "bracket set", script: "echo [ab]*.txt\n", want: "alpha.txt beta.txt\n"},
		{name: "a directory segment", script: "echo sub/*.txt\n", want: "sub/inner.txt\n"},
		{name: "a globbed directory segment", script: "echo s*/inner.txt\n", want: "sub/inner.txt\n"},
		{name: "matches are sorted", script: "echo b*.txt a*.txt\n", want: "beta.txt alpha.txt\n"},
		{name: "no match stays as written", script: "echo nosuch*.zzz\n", want: "nosuch*.zzz\n"},
		{name: "double quotes suppress it", script: "echo \"*.txt\"\n", want: "*.txt\n"},
		{name: "single quotes suppress it", script: "echo '*.txt'\n", want: "*.txt\n"},
		{name: "a backslash suppresses it", script: "echo \\*.txt\n", want: "*.txt\n"},
		{name: "an expansion result is globbed", script: "p='*.md'\necho $p\n", want: "gamma.md\n"},
		{name: "a quoted expansion result is not", script: "p='*.md'\necho \"$p\"\n", want: "*.md\n"},
		{name: "set -f turns it off", script: "set -f\necho *.txt\n", want: "*.txt\n"},
		{name: "a for loop iterates over matches", script: "for f in *.txt; do echo item=$f; done\n", want: "item=alpha.txt\nitem=beta.txt\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runCdScriptWithSeed(t, seedGlobTree+test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

func runCdScriptWithSeed(t *testing.T, source string) (int, string, string) {
	t.Helper()
	status, stdout, stderr, _ := runCdScript(t, source)
	return status, stdout, stderr
}
