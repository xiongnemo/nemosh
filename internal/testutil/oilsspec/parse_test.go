package oilsspec_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/testutil/oilsspec"
)

// Every vendored file parses, into as many cases as Oils counts in it. The parser is a
// port of test/sh_spec.py's, and when it was written its reading of all 2781 cases was
// compared with the original's, field for field; this keeps the count honest after.
func TestParse_readsEveryVendoredFileAsOilsCountsIt(t *testing.T) {
	record := readUpstream(t)
	files := 0
	for name, file := range record.Files {
		if !strings.HasSuffix(name, ".test.sh") {
			continue
		}
		files++
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		spec, err := oilsspec.Parse(string(data))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(spec.Cases) != file.Cases {
			t.Errorf("%s: parsed %d cases, and Oils counts %d", name, len(spec.Cases), file.Cases)
		}
	}
	if files == 0 {
		t.Fatal("upstream.json lists no spec files")
	}
}

func TestParse_readsTheGrammarAsOilsDoes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []oilsspec.Case
	}{{
		name:  "a one-line stdout stands for the line printed, newline and all",
		input: "## compare_shells: bash dash\n\n#### echo\necho hi\n## stdout: hi\n",
		want:  []oilsspec.Case{{Desc: "echo", Line: 3, Code: "echo hi\n", Default: map[string]string{"stdout": "hi\n"}}},
	}, {
		name: "blank lines inside code and output are kept, and those around them are not",
		input: "#### blanks\n\n\necho a\n\necho b\n## STDOUT:\na\n\nb\n## END\n\n\n" +
			"#### next\ntrue\n",
		want: []oilsspec.Case{
			{Desc: "blanks", Line: 1, Code: "echo a\n\necho b\n", Default: map[string]string{"stdout": "a\n\nb\n"}},
			{Desc: "next", Line: 14, Code: "true\n", Default: map[string]string{}},
		},
	}, {
		name:  "a line starting with # is a comment in code and in output alike",
		input: "#### comments\necho a # kept\n  # dropped\necho '#b'\n## STDOUT:\na\n# dropped\n#b\n## END\n",
		want:  []oilsspec.Case{{Desc: "comments", Line: 1, Code: "echo a # kept\necho '#b'\n", Default: map[string]string{"stdout": "a\n"}}},
	}, {
		name:  "the next ## line ends a multi-line value as ## END would",
		input: "#### no end\nprintf 'x\\n'\n## STDOUT:\nx\n## status: 1\n",
		want:  []oilsspec.Case{{Desc: "no end", Line: 1, Code: "printf 'x\\n'\n", Default: map[string]string{"stdout": "x\n", "status": "1"}}},
	}, {
		name: "a qualified line sets its value for each shell it names",
		input: "#### qualified\nfoo\n## status: 127\n## N-I dash/ash status: 2\n## N-I dash/ash stdout-json: \"\"\n" +
			"## OK bash STDOUT:\nfoo\n## END\n",
		want: []oilsspec.Case{{
			Desc: "qualified", Line: 1, Code: "foo\n", Default: map[string]string{"status": "127"},
			Shells: map[string]*oilsspec.Qualified{
				"dash": {Qualifier: "N-I", Values: map[string]string{"status": "2", "stdout-json": `""`}},
				"ash":  {Qualifier: "N-I", Values: map[string]string{"status": "2", "stdout-json": `""`}},
				"bash": {Qualifier: "OK", Values: map[string]string{"stdout": "foo\n"}},
			},
		}},
	}, {
		name:  "code given on a ## code: line is taken as written, without a newline",
		input: "#### given code\n## code: echo hi\n## stdout: hi\n",
		want:  []oilsspec.Case{{Desc: "given code", Line: 1, Code: "echo hi", Default: map[string]string{"stdout": "hi\n"}}},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec, err := oilsspec.Parse(test.input)
			if err != nil {
				t.Fatal(err)
			}
			for i := range test.want {
				if test.want[i].Shells == nil {
					test.want[i].Shells = map[string]*oilsspec.Qualified{}
				}
			}
			if !reflect.DeepEqual(spec.Cases, test.want) {
				t.Errorf("cases\n got %+v\nwant %+v", spec.Cases, test.want)
			}
		})
	}
}

func TestParse_refusesWhatOilsRefuses(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"a ## line that is not metadata", "#### a\ntrue\n## stdout hi\n", "line 3: invalid ## line"},
		{"stdout and stdout-json for one shell", "#### a\ntrue\n## OK bash stdout: a\n## OK bash stdout-json: \"a\"\n", "line 4: duplicate spec"},
		{"two qualifiers for one shell", "#### a\ntrue\n## OK bash stdout: a\n## BUG bash status: 1\n", "line 4: inconsistent qualifier"},
		{"a value on the STDOUT: line", "#### a\ntrue\n## STDOUT: a\n", "line 3: got value"},
		{"a case without code", "#### a\n## stdout: x\n#### b\ntrue\n", "line 3: expected a line of code"},
		{"code before the first case", "true\n#### a\ntrue\n", "line 1: expected ####"},
		{"file metadata sh_spec.py does not know", "## flavour: bash\n#### a\ntrue\n", "invalid file metadata"},
		{"a qualifier on file metadata", "## OK bash compare_shells: bash\n#### a\ntrue\n", "line 1: file metadata takes no qualifier"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := oilsspec.Parse(test.input)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error %v, want one saying %q", err, test.want)
			}
		})
	}
}
