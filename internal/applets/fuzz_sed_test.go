package applets

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"unicode/utf8"
)

// A sed script is text a script hands over, often built from variables. This parses every
// generated script, and runs the ones that touch no file -- no r or w command and no w flag
// on s, since those read and write the files they name -- and have no branch or D, over
// generated input. Each has to end with output or an error, never a panic or a hang.
func FuzzSed(f *testing.F) {
	for _, seed := range [][2]string{
		{"s/a/b/g", "banana"}, {"/x/d", "a\nx\nb"}, {"1!G;h;$!d", "1\n2\n3"}, {"$!N;P;D", "a\nb\nc"},
		{":a;s/^.\\{1,3\\}$/ &/;ta", "x"}, {"y/abc/xyz/", "aabbcc"}, {"2,3p", "1\n2\n3\n4"}, {"/a/,/b/{s/./X/;}", "a\nm\nb"},
		{"s/\\(a\\)\\(b\\)/\\2\\1/", "ab"}, {"$=", "a\nb"}, {"a\\\nnew", "x"}, {"0~2d", "1\n2"}, {"s/x*/-/g", "abc"},
		{"q5", "a"}, {"n;d", "1\n2\n3"}, {"l", "a\tb"}, {"s/a/\\n/", "a"}, {"{", "a"}, {"s/a", "a"}, {"b nowhere", "a"},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Chdir(f.TempDir())
	f.Fuzz(func(t *testing.T, script, input string) {
		if !utf8.ValidString(script) || !utf8.ValidString(input) || len(script) > 128 || len(input) > 256 || fuzzLongNumber.MatchString(script) {
			t.Skip()
		}
		program, err := parseSedProgram([]string{script}, false, false)
		if err != nil {
			return
		}
		for _, command := range program.instructions {
			// No file, and no command that can go round for ever on its own -- `:a;ba` and
			// `G;D` are loops in every sed -- so a run that does not end is a defect.
			if strings.IndexByte("rwbtTD", command.action) >= 0 || command.substitute.writeName != "" {
				return
			}
		}
		var stdout, stderr bytes.Buffer
		_ = program.run(context.Background(), nil, strings.NewReader(input), &stdout, &stderr)
	})
}
