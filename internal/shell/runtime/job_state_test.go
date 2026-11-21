package runtime

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// jobStateCoverage is the decision, for every field of Runtime, about what a background job
// that is a process gets of it. clone() and the codec each answer that question, and a field
// one of them forgets is a job that silently differs from a subshell; a field added to
// Runtime and not to this table fails the test until someone decides.
var jobStateCoverage = map[string]string{
	"initErr":           "rebuilt: the child makes its own runtime and reports its own failure",
	"registry":          "rebuilt: the same binary has the same applets",
	"functions":         "encoded: Functions, printed, and FunctionFiles",
	"streams":           "rebuilt: the child's descriptors are its own handles",
	"fds":               "rebuilt: the child's descriptors are its own handles",
	"vars":              "encoded: Vars",
	"traps":             "encoded: Traps",
	"listedTraps":       "not inherited: a job starts with no traps, and lists none",
	"trapRunning":       "not inherited: nothing is running a trap in a new process",
	"signals":           "not inherited: a job starts with no traps, so its inbox is new; its signals come over Control",
	"params":            "encoded: Name, Positional, Function, Getopts",
	"options":           "encoded: Options, Invocation",
	"expansion":         "not inherited: per command; the line travels as Line",
	"aliases":           "encoded: Aliases",
	"aliasChain":        "encoded: AliasChain",
	"aliasView":         "encoded: AliasView",
	"trapStatus":        "not inherited: a job begins in no trap's action",
	"childCPU":          "not inherited: a process counts its own children",
	"history":           "not inherited: a job does not read a prompt",
	"completions":       "not inherited: a job completes no line",
	"bindings":          "not inherited: a job reads no line",
	"commands":          "rebuilt: the child finds its commands itself, as a new process does",
	"shadowed":          "not inherited: a command's own, put back when it returns",
	"dirStack":          "encoded: DirStack",
	"arrays":            "encoded: Indexed, Associative",
	"loops":             "not inherited: a job does not break its parent's loop",
	"special":           "encoded: Seconds, ShellPID, ShellPPID; RANDOM is reseeded, as a subshell's is not",
	"locals":            "not inherited: a job restores nothing to its caller",
	"frames":            "encoded: Frames",
	"scriptFile":        "encoded: ScriptFile",
	"substitutions":     "rebuilt: the child waits for its own",
	"traceTurn":         "not inherited: a job process writes its trace lines itself, in no pipeline's turn",
	"pipeStage":         "not inherited: a job process is a process, and a write into a pipe no one reads ends it alone",
	"appletFrames":      "rebuilt: the child makes its own as its runtime is made; a frame holds this process's streams",
	"noFieldSplit":      "not inherited: per word",
	"errExitSuppressed": "encoded: ErrExitSuppressed",
	"plainAssignment":   "not inherited: per assignment statement",
	"arithmeticCommand": "not inherited: per command",
	"arrayOperands":     "not inherited: per command",
	"operandQuoted":     "not inherited: per word",
	"starFields":        "not inherited: per word",
	"readonly":          "encoded: Readonly",
	"attributes":        "encoded: Attributes",
	"mutatedVars":       "not inherited: a record of the parent's own writes",
	"mask":              "encoded: Umask",
	"sourceDepth":       "encoded: SourceDepth",
	"functionDepth":     "encoded: FunctionDepth",
	"subshellDepth":     "encoded: SubshellDepth, and the job one deeper",
	"searchPath":        "not inherited: per command",
	"interactive":       "not inherited: a job is not a session",
	"paths":             "rebuilt: the process starts in the working directory",
	"env":               "rebuilt: the process starts with the environment",
	"jobScope":          "rebuilt: the child's jobs are its own",
	"lifecycle":         "rebuilt: the child's exit is its own",
}

// A field of the shellState a Runtime embeds is the Runtime's as much as its own are, and is
// decided here by its name as they are.
func TestJobState_everyRuntimeFieldIsDecided(t *testing.T) {
	var names []string
	runtime := reflect.TypeFor[Runtime]()
	for index := range runtime.NumField() {
		if field := runtime.Field(index); field.Anonymous && field.Type == reflect.TypeFor[*shellState]() {
			shared := field.Type.Elem()
			for inner := range shared.NumField() {
				names = append(names, shared.Field(inner).Name)
			}
			continue
		}
		names = append(names, runtime.Field(index).Name)
	}
	seen := map[string]bool{}
	for _, name := range names {
		seen[name] = true
		if _, ok := jobStateCoverage[name]; !ok {
			t.Errorf("Runtime.%s has no entry in jobStateCoverage: decide whether a job process inherits it", name)
		}
	}
	for name := range jobStateCoverage {
		if !seen[name] {
			t.Errorf("jobStateCoverage names %s, which Runtime no longer has", name)
		}
	}
}

func TestJobState_everyOptionIsCarried(t *testing.T) {
	fields := reflect.TypeFor[shellOptions]()
	flags := shellOptionFields(&shellOptions{})
	bools := 0
	for index := range fields.NumField() {
		field := fields.Field(index)
		if field.Type.Kind() != reflect.Bool {
			continue
		}
		bools++
		if _, ok := flags[field.Name]; !ok {
			t.Errorf("shellOptions.%s is not in shellOptionFields, so a job would lose it", field.Name)
		}
	}
	if bools != len(flags) {
		t.Errorf("shellOptionFields has %d entries for %d flags", len(flags), bools)
	}
}

// A runtime with every kind of state set, captured, encoded, decoded and restored into a
// fresh one, answers a probe of all of it exactly as the original does, and hands back the
// job's program.
// A job started inside an alias's text is still inside it, and does not substitute that alias
// again: so `alias x='x &'` starts one job, where each job would otherwise start the next.
func TestJobState_carriesTheAliasChain(t *testing.T) {
	ctx := context.Background()
	original := New(applets.DefaultRegistry, Streams{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)})
	original.aliasChain = []string{"x", "y"}
	job, err := ParseScript("x\n")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeJobState(encodeJobState(original.captureJobState(job.program[0])))
	if err != nil {
		t.Fatal(err)
	}
	restored := New(applets.DefaultRegistry, Streams{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)})
	if _, err := restored.restoreJobState(ctx, decoded); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if !reflect.DeepEqual(restored.aliasChain, original.aliasChain) {
		t.Fatalf("the job's alias chain is %q, want %q", restored.aliasChain, original.aliasChain)
	}
}

func TestJobState_roundTripsEveryKindOfState(t *testing.T) {
	setup := `x=plain; export E=exported; readonly R=fixed
declare -i n=5; declare -l low=ABC; declare -x pending
a=(p "q r"); a[5]=s; unset 'a[1]'
declare -A m=([k]=v [j]=w)
declare -a da; declare -A dm
alias ll='ls -l'
set -o pipefail -u
shopt -s nullglob
umask 027
trap 'echo int' INT
f() {
  case $1 in
    x) echo "x $2" ;;
  esac
  cat <<END
body $1
END
}
set -- one "two words"
`
	probe := `echo "$x $E $R $n $low"; declare -p a m da dm n low pending 2>&1; alias; set -o | grep -E 'pipefail|nounset'; shopt nullglob; umask; trap -p INT; declare -f f; echo "$# $1 $2"; (R=changed) 2>/dev/null || echo "R held"`
	ctx := context.Background()
	original := New(applets.DefaultRegistry, Streams{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)})
	if status := original.RunScript(ctx, setup); status != 0 {
		t.Fatalf("setup exited %d", status)
	}
	job, err := ParseScript("echo from-the-job $1\n")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeJobState(encodeJobState(original.captureJobState(job.program[0])))
	if err != nil {
		t.Fatal(err)
	}
	restored := New(applets.DefaultRegistry, Streams{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)})
	program, err := restored.restoreJobState(ctx, decoded)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	run := func(rt Runtime, script string) string {
		var stdout bytes.Buffer
		rt.streams.Stdout = &stdout
		rt.fds = newFDTable(Streams{Stdout: &stdout, Stderr: &stdout})
		rt.streams = rt.fds.streams()
		rt.runScript(ctx, script, false)
		return stdout.String()
	}
	want, got := run(original, probe), run(restored, probe)
	if !strings.Contains(want, "trap -- 'echo int' SIGINT") || !strings.Contains(want, "R held") {
		t.Fatalf("the probe did not see the state it was written to check: %q", want)
	}
	if want != got {
		t.Fatalf("the restored runtime answers differently\n want: %q\n  got: %q", want, got)
	}
	var stdout bytes.Buffer
	table := newFDTable(Streams{Stdout: &stdout, Stderr: &stdout})
	restored.withFDTable(table).executeTypedScriptFrom(ctx, program, 0)
	if stdout.String() != "from-the-job one\n" {
		t.Fatalf("the job's program ran as %q", stdout.String())
	}
}
