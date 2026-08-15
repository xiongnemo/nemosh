package capability_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/capability"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// refusedTheOption reports whether an applet turned the option down, as opposed
// to failing for some unrelated reason.
//
// Running an applet for real is the only way to bind a table to behaviour, and
// most of them fail here for reasons that are not about options: a missing
// operand, an unreadable file. Those are not the question. The question is
// narrow -- did it say it does not know this option -- so the wording each
// applet uses for that, and only that, is what is matched.
//
// The word "option" has to appear. Without that requirement `date -d <dir>` read
// as a refused option, when what it actually said was that a directory is not a
// date: the option was accepted and its argument was not. Matching the verb
// alone made this test report a table error that was not one.
func refusedTheOption(output string) bool {
	if !strings.Contains(output, "option") {
		return false
	}
	for _, verb := range []string{"unsupported", "invalid", "unrecognized"} {
		if strings.Contains(output, verb) {
			return true
		}
	}
	return false
}

func runWithOption(t *testing.T, name, option string) string {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup(name)
	if !ok {
		t.Fatalf("applet %s is in the table but not in the registry", name)
	}
	var stdout, stderr bytes.Buffer
	// A temporary directory as the operand where one is wanted, so an applet
	// that reads or writes has something harmless to work on.
	err := applet.Run(context.Background(), []string{option, t.TempDir()},
		strings.NewReader(""), &stdout, &stderr)
	reported := stderr.String()
	if err != nil {
		reported += err.Error()
	}
	return reported
}

// Every option the table claims must actually be accepted.
//
// This is what stops the table becoming a second, stale version of the truth.
// It runs the applet rather than reading its source, so an option removed from
// the code fails here even though both files still look consistent.
func TestDeclaredOptionsAreAccepted(t *testing.T) {
	for _, name := range appletNames(t) {
		if launchesSomething[name] {
			continue
		}
		command, _ := capability.Lookup(name)
		for _, flag := range command.Short {
			t.Run(name+" -"+string(flag), func(t *testing.T) {
				if reported := runWithOption(t, name, "-"+string(flag)); refusedTheOption(reported) {
					t.Fatalf("%s claims -%c and refused it: %s", name, flag, reported)
				}
			})
		}
		for _, long := range command.Long {
			t.Run(name+" --"+long, func(t *testing.T) {
				if reported := runWithOption(t, name, "--"+long); refusedTheOption(reported) {
					t.Fatalf("%s claims --%s and refused it: %s", name, long, reported)
				}
			})
		}
	}
}

// Applets this test must not run, because running them starts a process.
//
// su is the only one. Every option in its row would launch something: -t starts
// a shell and waits for it, and -W or -c under the runas verb raises a consent
// dialog that no test can answer. Its options are held instead by
// TestPlanElevation_assemblesTheCommandLine and TestPlanElevation_refuses in
// internal/applets, which measure the same claims -- every option accepted,
// -Z refused -- against the same code, without starting anything.
//
// The registry only carries su on Windows, so elsewhere this map is never
// consulted.
var launchesSomething = map[string]bool{"su": true}

// And an option the table does not claim must be refused, or the claim is not
// saying anything. An applet that accepts everything would pass the test above
// while the table told the user nothing true.
//
// `-Z` is used because no applet here implements it and it is not a digit, so it
// cannot be mistaken for a count.
func TestUndeclaredOptionsAreRefused(t *testing.T) {
	// Applets that do not refuse an unknown option *as an option*, each for a
	// reason of its own, all of them recorded in docs/support-matrix.md's third
	// column. They claim no options either, so there is nothing to contradict --
	// this list exempts them from the contradiction test, not from the table.
	//
	// chmod, sed and pwd are here because measurement disagreed with that column
	// and measurement wins. chmod and sed have no option parsing at all: the
	// first operand is the mode and the script respectively, so `-Z` comes back
	// as `invalid mode '-Z'` and `unsupported sed script: -Z`. `pwd -Z` simply
	// succeeds, which the column does say.
	noOptionParsing := map[string]bool{
		"echo": true, "printf": true, "yes": true, "true": true, "false": true,
		"test": true, "[": true, "sleep": true, "env": true, "printenv": true,
		"posixpath": true, "realpath": true, "winpath": true,
		"chmod": true, "pwd": true, "sed": true,
		// seq's operands are numbers and a negative number begins with a dash,
		// so `-Z` is refused as a bad number rather than as a bad option --
		// which is the right refusal, just not the one this test looks for.
		"seq": true,
		// find refuses it, but as an unsupported *expression* rather than an
		// option, so the wording this test looks for is deliberately absent.
		"find": true,
	}
	for _, name := range appletNames(t) {
		if noOptionParsing[name] || launchesSomething[name] {
			continue
		}
		command, _ := capability.Lookup(name)
		if command.AcceptsShort('Z') {
			continue
		}
		t.Run(name, func(t *testing.T) {
			if reported := runWithOption(t, name, "-Z"); !refusedTheOption(reported) {
				t.Fatalf("%s accepted -Z, which the table does not claim: %q", name, reported)
			}
		})
	}
}

// The table must cover exactly the applets that exist. A new applet with no row
// would be drawn as an unknown command and complete nothing.
func TestEveryAppletHasARow(t *testing.T) {
	for _, name := range applets.DefaultRegistry.Names() {
		if !capability.Known(name) {
			t.Errorf("applet %s has no row in the capability table", name)
		}
	}
}

// And exactly the builtins that exist, which is the half this package cannot
// measure and so must at least keep in step by name.
func TestEveryBuiltinHasARow(t *testing.T) {
	for _, name := range runtime.BuiltinNames() {
		if !capability.Known(name) {
			t.Errorf("builtin %s has no row in the capability table", name)
		}
	}
}

func TestOptionsAreOfferedWithTheirDashes(t *testing.T) {
	// When
	got := capability.Options("ls")

	// Then
	want := []string{"-a", "-h", "-l", "-1", "--color"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("Options(ls) = %v, want %v", got, want)
	}
	if capability.Options("no-such-command") != nil {
		t.Fatal("an unknown command offers no options")
	}
}

func TestOperandKind(t *testing.T) {
	for _, test := range []struct {
		name string
		want capability.OperandKind
	}{
		{name: "cd", want: capability.Directory},
		{name: "mkdir", want: capability.Directory},
		{name: "rmdir", want: capability.Directory},
		{name: "ls", want: capability.AnyPath},
		{name: "cat", want: capability.AnyPath},
		{name: "not-a-command", want: capability.AnyPath},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := capability.OperandKindOf(test.name); got != test.want {
				t.Fatalf("OperandKindOf(%q) = %v, want %v", test.name, got, test.want)
			}
		})
	}
}

func appletNames(t *testing.T) []string {
	t.Helper()
	names := applets.DefaultRegistry.Names()
	if len(names) == 0 {
		t.Fatal("the registry is empty")
	}
	return names
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// The table's value is that it is measured, and exactly one row is not. This
// holds that line: an unmeasured row has to say so, and the ones that say so
// have to stay the ones we decided on.
//
// Without it, `External: true` would be an escape hatch that quietly widens --
// a row nobody can check is easier to write than one that has to survive being
// run, so the pressure is always in that direction.
func TestOnlyExternalRowsAreUnmeasured(t *testing.T) {
	// Given: the names the drift tests above actually exercise.
	measured := map[string]bool{}
	for _, name := range appletNames(t) {
		measured[name] = true
	}

	// When
	var unmeasured []string
	for _, name := range capability.Names() {
		command, _ := capability.Lookup(name)
		if command.Builtin || measured[name] || launchesSomething[name] {
			continue
		}
		if !command.External {
			t.Errorf("%s is neither an applet, a builtin, nor marked External: nothing measures it", name)
		}
		unmeasured = append(unmeasured, name)
	}

	// Then: the list of rows nobody can check is the one that was argued for,
	// command by command, in commands.go.
	if !slices.Equal(unmeasured, []string{"ssh"}) {
		t.Fatalf("unmeasured rows = %v, want only [ssh]. Adding one is a decision, not a detail", unmeasured)
	}
}

// An option that takes a file must take a value, and one that takes a value must
// be an option the command accepts. Two subsets, stated as such, so a letter
// cannot be claimed in the narrow column and missing from the wide one.
func TestValueAndFileOptionsAreSubsetsOfTheAcceptedOnes(t *testing.T) {
	for _, name := range capability.Names() {
		command, _ := capability.Lookup(name)
		t.Run(name, func(t *testing.T) {
			for _, flag := range command.ValueShort {
				if !command.AcceptsShort(flag) {
					t.Errorf("%s claims -%c takes a value but does not accept it", name, flag)
				}
			}
			for _, flag := range command.FileShort {
				if !command.TakesValue(flag) {
					t.Errorf("%s claims -%c takes a file but not a value", name, flag)
				}
			}
		})
	}
}
