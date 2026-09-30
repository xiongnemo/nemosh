package runtime_test

import "testing"

// type and command -V call a special builtin special, as busybox does, and busybox's own
// builtins that are applets here shell builtins. They said "eval is a shell builtin" and
// "echo is a builtin applet".
func TestRuntime_typeCallsASpecialBuiltinSpecial(t *testing.T) {
	script := "type cd eval : true export local times echo printf [ ls; command -V source\n"
	want := "cd is a shell builtin\n" +
		"eval is a special shell builtin\n" +
		": is a special shell builtin\n" +
		"true is a shell builtin\n" +
		"export is a special shell builtin\n" +
		"local is a special shell builtin\n" +
		"times is a special shell builtin\n" +
		"echo is a shell builtin\n" +
		"printf is a shell builtin\n" +
		"[ is a shell builtin\n" +
		"ls is a builtin applet\n" +
		"source is a special shell builtin\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Errorf("got %q/%d, want %q/0, as busybox answers", stdout, status, want)
	}
}
