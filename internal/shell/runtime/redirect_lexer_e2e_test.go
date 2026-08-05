package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_redirectOperandsUseOrdinaryQuoteAndEscapeScanning(t *testing.T) {
	tmp := t.TempDir()
	doubleQuoted := filepath.ToSlash(filepath.Join(tmp, "double quoted.txt"))
	singleQuoted := filepath.ToSlash(filepath.Join(tmp, "single quoted.txt"))
	escaped := filepath.ToSlash(filepath.Join(tmp, "escaped name.txt"))
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})
	script := "file='" + doubleQuoted + "'\n" +
		"echo double >\"$file\"\n" +
		"echo single >'" + singleQuoted + "'\n" +
		"echo escaped >" + escapeSpaces(escaped) + "\n"

	if status := rt.RunScript(context.Background(), script); status != 0 {
		t.Fatalf("status: %d", status)
	}
	assertFileText(t, doubleQuoted, "double\n")
	assertFileText(t, singleQuoted, "single\n")
	assertFileText(t, escaped, "escaped\n")
}

func TestRuntime_appendRedirectOperandUsesOrdinaryExpansion(t *testing.T) {
	// Given
	tmp := t.TempDir()
	output := filepath.ToSlash(filepath.Join(tmp, "expanded append.txt"))
	if err := os.WriteFile(output, []byte("before\n"), 0o600); err != nil {
		t.Fatalf("seed output: %v", err)
	}
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	// When
	status := rt.RunScript(context.Background(), "file='"+output+"'\necho after >>\"$file\"\n")

	// Then
	if status != 0 {
		t.Fatalf("status: %d", status)
	}
	assertFileText(t, output, "before\nafter\n")
}

func TestRuntime_singleQuotedRedirectOperandSuppressesExpansion(t *testing.T) {
	t.Chdir(t.TempDir())
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	if status := rt.RunScript(context.Background(), "file=expanded\necho literal >'$file'\n"); status != 0 {
		t.Fatalf("status: %d", status)
	}
	assertFileText(t, "$file", "literal\n")
	if _, err := os.Stat("expanded"); !os.IsNotExist(err) {
		t.Fatalf("expanded redirect target exists: %v", err)
	}
}

func TestRuntime_escapedRedirectOperandSuppressesExpansion(t *testing.T) {
	t.Chdir(t.TempDir())
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	if status := rt.RunScript(context.Background(), "file=expanded\necho literal >\\$file\n"); status != 0 {
		t.Fatalf("status: %d", status)
	}
	assertFileText(t, "$file", "literal\n")
	if _, err := os.Stat("expanded"); !os.IsNotExist(err) {
		t.Fatalf("expanded redirect target exists: %v", err)
	}
}

func TestRuntime_emptyQuotedRedirectOperandIsPresent(t *testing.T) {
	for _, operand := range []string{"''", `""`} {
		t.Run(operand, func(t *testing.T) {
			var stderr bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stderr: &stderr})

			if status := rt.RunScript(context.Background(), "echo value >"+operand+"\n"); status == 0 {
				t.Fatal("expected opening an empty path to fail")
			}
			if bytes.Contains(stderr.Bytes(), []byte("missing redirection target")) {
				t.Fatalf("quoted empty operand was discarded: %s", stderr.String())
			}
		})
	}
}

func TestRuntime_preservesAdjacentRedirectOrder(t *testing.T) {
	tmp := t.TempDir()
	input := filepath.ToSlash(filepath.Join(tmp, "in.txt"))
	output := filepath.ToSlash(filepath.Join(tmp, "out.txt"))
	if err := os.WriteFile(input, []byte("adjacent\n"), 0o600); err != nil {
		t.Fatalf("input: %v", err)
	}
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	if status := rt.RunScript(context.Background(), "cat<"+input+">"+output+"\n"); status != 0 {
		t.Fatalf("status: %d", status)
	}
	assertFileText(t, output, "adjacent\n")
}

func TestRuntime_quotedAndEscapedIONumbersRemainCommandArguments(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		operand    string
		wantStdout string
		wantFile   string
	}{
		{name: "unquoted digit redirects descriptor two", operand: `2`, wantStdout: "\n", wantFile: ""},
		{name: "double quoted digit is an argument", operand: `"2"`, wantFile: "2\n"},
		{name: "single quoted digit is an argument", operand: `'2'`, wantFile: "2\n"},
		{name: "escaped digit is an argument", operand: `\2`, wantFile: "2\n"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			output := filepath.ToSlash(filepath.Join(t.TempDir(), "out.txt"))
			var stdout bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

			status := rt.RunScript(context.Background(), "echo "+testCase.operand+">"+output+"\n")

			if status != 0 {
				t.Fatalf("status: %d", status)
			}
			if stdout.String() != testCase.wantStdout {
				t.Fatalf("stdout: got %q want %q", stdout.String(), testCase.wantStdout)
			}
			assertFileText(t, output, testCase.wantFile)
		})
	}
}

func TestRuntime_rejectsRedirectExpansionWithMultipleFieldsBeforeExecution(t *testing.T) {
	probe := filepath.ToSlash(filepath.Join(t.TempDir(), "probe.txt"))
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stderr: &stderr})

	status := rt.RunScript(context.Background(), "set -- first second\necho ran >$@ >"+probe+"\n")

	if status == 0 {
		t.Fatal("expected ambiguous redirect")
	}
	if _, err := os.Stat(probe); !os.IsNotExist(err) {
		t.Fatalf("command executed: %v", err)
	}
}

func TestRuntime_redirectExpansionCannotCreateRedirectSyntax(t *testing.T) {
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "output.txt"))
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	if status := rt.RunScript(context.Background(), "syntax='>literal'\necho $syntax >"+path+"\n"); status != 0 {
		t.Fatalf("status: %d", status)
	}
	assertFileText(t, path, ">literal\n")
}

func TestRuntime_trailingBackslashFailsBeforeExecution(t *testing.T) {
	probe := filepath.ToSlash(filepath.Join(t.TempDir(), "probe.txt"))
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	status := rt.RunScript(context.Background(), "echo ran >"+probe+" \\")

	if status == 0 {
		t.Fatal("expected lexical failure")
	}
	if _, err := os.Stat(probe); !os.IsNotExist(err) {
		t.Fatalf("command executed: %v", err)
	}
}

func escapeSpaces(value string) string {
	result := make([]byte, 0, len(value))
	for index := range len(value) {
		if value[index] == ' ' {
			result = append(result, '\\')
		}
		result = append(result, value[index])
	}
	return string(result)
}
