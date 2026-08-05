package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestSubshellCommand_isolatesAllMutableRuntimeState(t *testing.T) {
	var stdout bytes.Buffer
	runtime := New(applets.DefaultRegistry, Streams{Stdout: &stdout})
	runtime.vars["VALUE"] = "parent"
	runtime.env.Set("EXPORTED", "parent")
	runtime.traps["EXIT"] = "echo parent-trap"
	runtime.readonly["PARENT_READONLY"] = struct{}{}
	runtime.mask.value = 0o022
	runtime.params.values = []string{"parent"}

	script, err := ParseScript("(VALUE=child; export EXPORTED=child; trap 'echo child-trap' EXIT; readonly CHILD_READONLY=1; umask 077; set -- child; exec true)\n")
	if err != nil {
		t.Fatal(err)
	}
	status, control := runtime.executePrepared(context.Background(), script)

	exported, exists := runtime.env.LookupEnv("EXPORTED")
	if status != 0 || control != flowNone || runtime.vars["VALUE"] != "parent" || !exists || exported != "parent" {
		t.Fatalf("status = %d, control = %d, vars = %#v, env = %q", status, control, runtime.vars, exported)
	}
	if runtime.traps["EXIT"] != "echo parent-trap" || runtime.mask.value != 0o022 || runtime.params.values[0] != "parent" {
		t.Fatalf("trap = %q, mask = %o, params = %#v", runtime.traps["EXIT"], runtime.mask.value, runtime.params.values)
	}
	if _, exists := runtime.readonly["CHILD_READONLY"]; exists {
		t.Fatal("child readonly registry escaped subshell")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestSubshellCommand_isolatesFDMapping(t *testing.T) {
	var output bytes.Buffer
	runtime := New(applets.DefaultRegistry, Streams{})
	if err := runtime.fds.bindBorrowedWriter(3, &output); err != nil {
		t.Fatal(err)
	}

	status := runtime.RunScript(context.Background(), "(echo child >&3; exec 3>&-)\necho parent >&3\n")

	if status != 0 || output.String() != "child\nparent\n" {
		t.Fatalf("status = %d, fd output = %q", status, output.String())
	}
}

func TestGroupCommands_returnLastBodyStatus(t *testing.T) {
	for _, source := range []string{"{ true; false; }\n", "(true; false)\n"} {
		runtime := New(applets.DefaultRegistry, Streams{})
		if status := runtime.RunScript(context.Background(), source); status != 1 {
			t.Fatalf("RunScript(%q) status = %d, want 1", source, status)
		}
	}
}

func TestRuntime_RunScript_hasNoPrefixEffects_whenGroupSyntaxIsMalformed(t *testing.T) {
	var stdout bytes.Buffer
	runtime := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

	status := runtime.RunScript(context.Background(), "echo prefix\n{ echo malformed }\n")

	if status != 2 || stdout.Len() != 0 {
		t.Fatalf("status = %d, stdout = %q", status, stdout.String())
	}
}

func TestRuntime_RunScript_hasNoPrefixEffects_whenGroupDelimitersAreCrossed(t *testing.T) {
	var stdout bytes.Buffer
	runtime := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

	status := runtime.RunScript(context.Background(), "echo prefix\n( { echo mixed; ) echo leaked; }\n")

	if status != 2 || stdout.Len() != 0 {
		t.Fatalf("status = %d, stdout = %q", status, stdout.String())
	}
}
