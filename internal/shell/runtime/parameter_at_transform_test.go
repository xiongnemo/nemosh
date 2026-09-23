package runtime_test

import (
	"strings"
	"testing"
)

// `${name@op}`, bash's parameter transformations. Each was `bad substitution`. Every answer
// is bash's, measured; busybox has none.
func TestParameterTransform_answersAsBashDoes(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "Q quotes for reuse", script: "x=\"a b'c\"\necho \"${x@Q}\"\n", want: "'a b'\\''c'\n"},
		{name: "Q of an empty value", script: "x=\necho \"[${x@Q}]\"\n", want: "['']\n"},
		{name: "Q of an unset name is nothing", script: "unset z\necho \"[${z@Q}]\"\n", want: "[]\n"},
		{name: "Q round-trips through eval", script: "x=\"it's\"\neval \"y=${x@Q}\"\necho \"$y\"\n", want: "it's\n"},
		{name: "E decodes escapes", script: "x='a\\tb'\necho \"${x@E}\"\n", want: "a\tb\n"},
		{name: "case", script: "x='hello World'\necho \"${x@U} ${x@u} ${x@L}\"\n", want: "HELLO WORLD Hello World hello world\n"},
		{name: "A of a plain name", script: "x=1\necho \"${x@A}\"\n", want: "x='1'\n"},
		{name: "A carries the attributes", script: "export x=1\nreadonly r=2\necho \"${x@A} ${r@A}\"\n", want: "declare -x x='1' declare -r r='2'\n"},
		{name: "a lists them", script: "x=1\nexport x\nreadonly x\na=(1)\ndeclare -A m\necho \"[${x@a}] [${a@a}] [${m@a}]\"\n", want: "[rx] [a] [A]\n"},
		{name: "an element", script: "a=(x y)\necho \"${a[1]@Q}\"\n", want: "'y'\n"},
		{name: "every element, one field each", script: "a=(p 'q r')\nfor w in \"${a[@]@Q}\"; do echo \"<$w>\"; done\n", want: "<'p'>\n<'q r'>\n"},
		{name: "the positional parameters", script: "set -- a 'b c'\necho \"${@@Q}\"\n", want: "'a' 'b c'\n"},
		{name: "a star form joins", script: "a=(x y)\necho \"${a[*]@U}\"\n", want: "X Y\n"},
		{name: "A of an array is its declaration", script: "a=(1 '2 3')\necho \"${a[@]@A}\"\n", want: "declare -a a=([0]=\"1\" [1]=\"2 3\")\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// `@P` needs the prompt's own escapes, which the line editor draws; it is refused by name
// rather than done by half.
func TestParameterTransform_refusesPromptExpansionByName(t *testing.T) {
	stdout, status := runScriptCapturing("x=abc\necho \"${x@P}\"\n")
	if status != 2 || strings.Contains(stdout, "abc") {
		t.Fatalf("got %q/%d, want a refusal", stdout, status)
	}
}
