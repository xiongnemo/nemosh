package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestParseScript_buildsTypedFunctionDefinition(t *testing.T) {
	// Given
	source := "greet() { echo hello; } >out.txt\n"

	// When
	script, err := ParseScript(source)

	// Then
	if err != nil {
		t.Fatalf("ParseScript() error = %v", err)
	}
	if len(script.program) != 1 {
		t.Fatalf("program length = %d, want 1", len(script.program))
	}
	definition, ok := script.program[0].(functionDefinition)
	if !ok {
		t.Fatalf("program node = %T, want functionDefinition", script.program[0])
	}
	if definition.name.String() != "greet" {
		t.Fatalf("function name = %q, want greet", definition.name.String())
	}
	if _, ok := definition.body.(braceGroup); !ok {
		t.Fatalf("function body = %T, want braceGroup", definition.body)
	}
}

func TestRuntime_definesWithoutExecuting_andRedefinesFunction(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "f() { echo old; }\nf() { echo new; }\nf\n")

	// Then
	if status != 0 || stdout.String() != "new\n" {
		t.Fatalf("RunScript() = status %d, stdout %q; want 0, new", status, stdout.String())
	}
}

func TestRuntime_functionCall_replacesAndRestoresPositionalParameters(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})
	source := "set -- caller\nf() { printf '<%s><%s><%s>' \"$#\" \"$1\" \"$2\"; printf '<%s><%s>' \"$@\"; }\nf one 'two words'\nprintf '<%s><%s>' \"$#\" \"$1\"\n"

	// When
	status := rt.RunScript(context.Background(), source)

	// Then
	want := "<2><one><two words><one><two words><1><caller>"
	if status != 0 || stdout.String() != want {
		t.Fatalf("RunScript() = status %d, stdout %q; want 0, %q", status, stdout.String(), want)
	}
}

func TestRuntime_functionReturn_stopsNearestNestedCall(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	source := "inner() { echo inner; false; return; echo no; }\nouter() { inner || echo seven; echo outer; return 3; echo no; }\nouter || echo recovered\necho after\n"

	// When
	status := rt.RunScript(context.Background(), source)

	// Then
	if status != 0 || stdout.String() != "inner\nseven\nouter\nrecovered\nafter\n" {
		t.Fatalf("RunScript() = status %d, stdout %q", status, stdout.String())
	}
}

func TestRuntime_functionRedirectAndHeredoc_applyAtCallTime(t *testing.T) {
	// Given
	dir := t.TempDir()
	var stdout bytes.Buffer
	state := State{Cwd: WorkingDirectory(dir), Env: NewEnvironment(nil)}
	rt := NewWithState(applets.DefaultRegistry, Streams{Stdout: &stdout}, state)
	source := "file=one.txt\nf() { cat; } >\"$file\" <<EOF\nhello $file\nEOF\nfile=two.txt\nf\ncat two.txt\n"

	// When
	status := rt.RunScript(context.Background(), source)

	// Then
	if status != 0 || stdout.String() != "hello two.txt\n" {
		t.Fatalf("RunScript() = status %d, stdout %q", status, stdout.String())
	}
	if filepath.Clean(rt.resolvePath("two.txt")) == "" {
		t.Fatal("expected resolved output path")
	}
}

func TestRuntime_functionRegistry_isIsolatedInSubshellAndPipeline(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	source := "f() { echo parent; }\n( f; f() { echo child; }; f )\nf\nf | cat\nf\n"

	// When
	status := rt.RunScript(context.Background(), source)

	// Then
	want := "parent\nchild\nparent\nparent\nparent\n"
	if status != 0 || stdout.String() != want {
		t.Fatalf("RunScript() = status %d, stdout %q, stderr %q; want 0, %q", status, stdout.String(), stderr.String(), want)
	}
}

func TestRuntime_malformedSuffixDoesNotInstallParsedPrefix(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	badStatus := rt.RunScript(context.Background(), "f() { echo installed; }\nfi\n")
	callStatus := rt.RunScript(context.Background(), "f\n")

	// Then
	if badStatus != 2 || callStatus == 0 || stdout.Len() != 0 {
		t.Fatalf("statuses = (%d, %d), stdout = %q; want (2, nonzero), empty", badStatus, callStatus, stdout.String())
	}
}

func TestRuntime_rejectsFunctionKeywordWithoutInstalling(t *testing.T) {
	// Given
	var stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "function f { echo no; }\n")

	// Then
	if status != 2 {
		t.Fatalf("RunScript() status = %d, want 2", status)
	}
}

func TestParseScript_acceptsSubshellFunctionBody_andRejectsNonPortableNames(t *testing.T) {
	// Given
	valid := "f() ( echo child )\n"
	invalid := []string{"1f() { echo no; }\n", "'f'() { echo no; }\n", "f-name() { echo no; }\n"}

	// When
	script, err := ParseScript(valid)

	// Then
	if err != nil {
		t.Fatalf("ParseScript(valid) error = %v", err)
	}
	definition := script.program[0].(functionDefinition)
	if _, ok := definition.body.(subshellCommand); !ok {
		t.Fatalf("function body = %T, want subshellCommand", definition.body)
	}
	for _, source := range invalid {
		if _, err := ParseScript(source); err == nil {
			t.Fatalf("ParseScript(%q) error = nil, want syntax error", source)
		}
	}
}

func TestParseScript_acceptsFunctionBodyOnFollowingLogicalLine(t *testing.T) {
	// Given
	sources := []string{
		"f()\n{ echo brace; }\nf\n",
		"f()\n( echo subshell )\nf\n",
	}

	for _, source := range sources {
		// When
		_, err := ParseScript(source)

		// Then
		if err != nil {
			t.Fatalf("ParseScript(%q) error = %v", source, err)
		}
	}
}

func TestRuntime_functionLookup_precedesRegularBuiltin_andCommandBypassesFunction(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})
	source := "echo() { printf function; }\necho ignored\ncommand echo applet\ncommand -v echo\n"

	// When
	status := rt.RunScript(context.Background(), source)

	// Then
	if status != 0 || stdout.String() != "functionapplet\necho\n" {
		t.Fatalf("RunScript() = status %d, stdout %q", status, stdout.String())
	}
}

func TestRuntime_functionCall_persistsShellState(t *testing.T) {
	// Given
	dir := t.TempDir()
	child := filepath.Join(dir, "child")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	var stdout bytes.Buffer
	rt := NewWithState(applets.DefaultRegistry, Streams{Stdout: &stdout}, State{Cwd: WorkingDirectory(dir), Env: NewEnvironment(nil)})
	source := "f() { value=kept; cd child; g() { echo nested; }; }\nf\necho $value\npwd\ng\n"

	// When
	status := rt.RunScript(context.Background(), source)

	// Then
	wantSuffix := "/child\nnested\n"
	if status != 0 || !bytes.HasPrefix(stdout.Bytes(), []byte("kept\n")) || !bytes.HasSuffix(stdout.Bytes(), []byte(wantSuffix)) {
		t.Fatalf("RunScript() = status %d, stdout %q", status, stdout.String())
	}
}

func TestRuntime_returnInsideLoopAndGroup_exitsFunction(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})
	source := "f() { for x in one two\ndo\n{ echo hit; return 6; }\ndone\necho no; }\nf || echo returned\necho after\n"

	// When
	status := rt.RunScript(context.Background(), source)

	// Then
	if status != 0 || stdout.String() != "hit\nreturned\nafter\n" {
		t.Fatalf("RunScript() = status %d, stdout %q", status, stdout.String())
	}
}

func TestRuntime_sourceAndFunctionReturn_consumeNearestBoundary(t *testing.T) {
	// Given
	dir := t.TempDir()
	library := filepath.Join(dir, "library.sh")
	if err := os.WriteFile(library, []byte("echo sourced\nreturn 4\necho source-no\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})
	source := "f() { . " + filepath.ToSlash(library) + " || echo source-returned; echo function; return 5; }\nf || echo function-returned\n"

	// When
	status := rt.RunScript(context.Background(), source)

	// Then
	want := "sourced\nsource-returned\nfunction\nfunction-returned\n"
	if status != 0 || stdout.String() != want {
		t.Fatalf("RunScript() = status %d, stdout %q; want 0, %q", status, stdout.String(), want)
	}
}

func TestRuntime_recursiveFunction_restoresEachCallFrame(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})
	source := "f() { echo $1; shift; if test \"$#\" = 0\nthen\nreturn\nfi\nf \"$@\"; echo frame; }\nf one two\n"

	// When
	status := rt.RunScript(context.Background(), source)

	// Then
	if status != 0 || stdout.String() != "one\ntwo\nframe\n" {
		t.Fatalf("RunScript() = status %d, stdout %q", status, stdout.String())
	}
}

func TestRuntime_recursiveFunction_stopsAtCallDepthLimit(t *testing.T) {
	// Given
	var stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "f() { f; }\nf\n")

	// Then
	if status == 0 || !bytes.Contains(stderr.Bytes(), []byte("function call depth")) {
		t.Fatalf("RunScript() = status %d, stderr %q; want nonzero depth diagnostic", status, stderr.String())
	}
}

func TestRuntime_functionInvocationRedirect_wrapsWholeBody(t *testing.T) {
	// Given
	dir := t.TempDir()
	var stdout bytes.Buffer
	rt := NewWithState(applets.DefaultRegistry, Streams{Stdout: &stdout}, State{Cwd: WorkingDirectory(dir), Env: NewEnvironment(nil)})
	source := "f() { echo one; echo two; }\nf >call.txt\ncat call.txt\n"

	// When
	status := rt.RunScript(context.Background(), source)

	// Then
	if status != 0 || stdout.String() != "one\ntwo\n" {
		t.Fatalf("RunScript() = status %d, stdout %q", status, stdout.String())
	}
}

func TestRuntime_functionRegistry_isVisibleAndIsolatedInCommandSubstitution(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})
	source := "f() { echo parent; }\necho $(f)\nf\n"

	// When
	status := rt.RunScript(context.Background(), source)

	// Then
	if status != 0 || stdout.String() != "parent\nparent\n" {
		t.Fatalf("RunScript() = status %d, stdout %q", status, stdout.String())
	}
}
