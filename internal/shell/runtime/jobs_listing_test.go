package runtime_test

import "testing"

// Where `jobs` can see the shell's jobs. A pipeline stage and a command substitution see
// them, and a subshell does not: busybox-w32 and bash 5.3 agree on each line here but one.
// `$(jobs -p | wc -l)` is 2 in bash and 0 in busybox, and this gives bash's, since it is
// the same substitution as `$(jobs -p)`, which both see into.
func TestJobs_listsTheShellsJobsInAStageOrASubstitution(t *testing.T) {
	status, stdout, stderr := runSetScript(t, "sleep 5 &\nsleep 5 &\n"+
		"jobs -p | wc -l\n"+
		"echo \"[$(jobs -p | wc -l)]\"\n"+
		"( jobs -p | wc -l )\n"+
		"kill $(jobs -p)\nwait\necho \"after=[$(jobs)]\"\n")
	if status != 0 || stdout != "2\n[2]\n0\nafter=[]\n" {
		t.Fatalf("status %d stdout %q stderr %q", status, stdout, stderr)
	}
}

// A substitution that lists the shell's jobs does not take their news from it: the shell
// still reports a finished job itself.
func TestJobs_aSubstitutionConsumesNothingOfTheShells(t *testing.T) {
	status, stdout, _ := runSetScript(t, "true &\nwait %1 2>/dev/null; sleep 0.1\nfalse &\nsleep 0.2\nseen=$(jobs)\njobs\n")
	if status != 0 || stdout != "[2] Done(1)\n" {
		t.Fatalf("status %d stdout %q", status, stdout)
	}
}
