package runtime_test

import "testing"

// `cmd | while read -r line; do ...; done` reported `unexpected do`.
//
// The span builder finds a compound only at the start of a line. That is the canonical
// way to process a command's output line by line, and with `done < file` fixed only in
// the previous round, there had been no direct spelling for reading input into a loop at
// all.
//
// The compound becomes a brace group, which is what it already becomes when a redirection
// follows its closer, and a brace group has been usable as a pipeline stage all along --
// so the work was in finding it, not in running it.
func TestPipelineCompound_takesACompoundAsAStage(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			// The idiom.
			name:   "piping into a while loop",
			script: "printf 'a\\nb\\n' | while read -r l; do printf '<%s>' \"$l\"; done\necho\n",
			want:   "<a><b>\n",
		},
		{
			name:   "two variables per line",
			script: "printf 'a b\\n' | while read -r x y; do echo \"$x-$y\"; done\n", want: "a-b\n",
		},
		{
			// Two pipes: everything before the last one is the prefix.
			name:   "through a second stage first",
			script: "echo x | cat | while read -r l; do echo \"got=$l\"; done\n", want: "got=x\n",
		},
		{name: "piping into an if", script: "true | if true; then echo piped; fi\n", want: "piped\n"},
		{name: "piping into a for", script: "true | for i in one; do echo \"$i\"; done\n", want: "one\n"},
		// Piping into a `case` is deliberately absent: it needs the case-arm line pass to
		// find a `case` after a pipe, which is a further layer in. Piping into a while,
		// if, for or until all work.
		{name: "piping into an until", script: "true | until true; do echo never; done\necho done\n", want: "done\n"},
		{
			// An and-or in front: the pipe binds tighter, so this is `true && (printf | while)`.
			name:   "after an and-if",
			script: "true && printf 'a\\n' | while read -r l; do echo \"$l\"; done\n", want: "a\n",
		},
		{
			name:   "break out of the piped loop",
			script: "printf '1\\n2\\n' | while read -r n; do if [ \"$n\" = 2 ]; then break; fi; echo \"n=$n\"; done\n",
			want:   "n=1\n",
		},
		{
			// A loop in a pipeline runs in a subshell, in bash and here, so the variable
			// does not survive. That falls out of using a brace group rather than being
			// arranged for.
			name:   "the loop is a subshell",
			script: "x=0\nprintf 'a\\n' | while read -r l; do x=1; done\necho \"x=$x\"\n", want: "x=0\n",
		},
		{
			name:   "piping out of the loop as well",
			script: "printf 'a\\n' | while read -r l; do echo \"$l\"; done | cat\n", want: "a\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}

// An operator after a closer: `done | cat` makes the compound the first stage of a
// pipeline, and `esac && echo` the first term of an and-or.
func TestPipelineCompound_takesAnOperatorAfterTheCloser(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "a loop into a pipe", script: "for i in a; do echo $i; done | cat\n", want: "a\n"},
		{name: "an if into a pipe", script: "if true; then echo t; fi | cat\n", want: "t\n"},
		{name: "a case into a pipe", script: "case a in a) echo c ;; esac | cat\n", want: "c\n"},
		{name: "two stages after", script: "for i in a b; do echo $i; done | cat | wc -l\n", want: "2\n"},
		{name: "and-if after a loop", script: "for i in a; do echo $i; done && echo and\n", want: "a\nand\n"},
		{name: "and-if after a case", script: "case a in a) echo c ;; esac && echo and\n", want: "c\nand\n"},
		{
			// The case matches nothing, so the status is 0 and `||` does not fire.
			name: "or-if after a case that succeeds", script: "case z in a) true ;; esac || echo or\necho end\n",
			want: "end\n",
		},
		{
			name:   "or-if after a failing loop",
			script: "for i in a; do false; done || echo or\n", want: "or\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}

// A `case` inside a brace group. Three scans had to learn that a `)` while a `}` is open
// is a pattern rather than a bracket: the one that decides where a logical line ends, the
// one that decides where a `;` splits, and the one that finds the group's extent.
func TestPipelineCompound_takesACaseInsideABraceGroup(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "on its own", script: "{ case a in a) echo c ;; esac; }\n", want: "c\n"},
		{name: "with the optional paren", script: "{ case a in (a) echo p ;; esac; }\n", want: "p\n"},
		{name: "then an and-if", script: "{ case a in a) echo c ;; esac; } && echo chained\n", want: "c\nchained\n"},
		{name: "into a pipe", script: "{ case a in a) echo c ;; esac; } | cat\n", want: "c\n"},
		// A *multi-arm* case inside a group is absent too: the second pattern's `)` meets
		// a fourth scan with its own opinion about brackets. A case at the top level
		// takes any number of arms, and a single-arm one works inside a group, so what is
		// left is the intersection of two uncommon spellings.
		{name: "a subshell inside an arm", script: "case a in a) (echo sub) ;; esac\n", want: "sub\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}

// Crossed delimiters must stay a *complete* syntax error rather than an incomplete one.
// The distinction is not cosmetic: an incomplete script makes an interactive shell wait
// for more of a line that can never be finished. Treating every `)` as a case pattern is
// what turned this into a hang, and it is why the three scans ask whether a `case` is
// actually open.
func TestPipelineCompound_stillRejectsCrossedDelimiters(t *testing.T) {
	// When
	_, _, stderr := runSetScript(t, "( { echo mixed; ) echo leaked; }\n")

	// Then
	if !contains(stderr, "unexpected )") {
		t.Fatalf("stderr = %q, want the crossed delimiters named", stderr)
	}
	if contains(stderr, "incomplete") {
		t.Fatalf("stderr = %q, want a complete syntax error: an incomplete one makes an "+
			"interactive shell wait for a line that cannot be finished", stderr)
	}
}

// The forms these scans exist for, which must be untouched.
func TestPipelineCompound_leavesOrdinaryPipelinesAlone(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "a plain pipeline", script: "echo a | cat\n", want: "a\n"},
		{name: "three stages", script: "echo a | cat | cat\n", want: "a\n"},
		{name: "a pipe in a quoted word", script: "echo 'a|b'\n", want: "a|b\n"},
		{name: "or-if is not a pipe", script: "false || echo or\n", want: "or\n"},
		{name: "a word beginning with a keyword", script: "echo iffy | cat\n", want: "iffy\n"},
		{name: "a group as a stage", script: "echo a | { read -r l; echo \"$l\"; }\n", want: "a\n"},
		{name: "a subshell as a stage", script: "echo a | (read -r l; echo \"$l\")\n", want: "a\n"},
		{name: "a plain compound", script: "for i in a; do echo $i; done\n", want: "a\n"},
		{name: "a compound with a redirect", script: "while read -r l; do echo x; done < /dev/null\necho ok\n", want: "ok\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}
