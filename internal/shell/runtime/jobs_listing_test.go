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
// still reports a finished job itself. The substitutions are asked until one has seen the
// job done, for ten seconds at most. A fixed `sleep 0.2` before a single one was a race the
// job lost on the arm64 runner, which listed it still Running.
func TestJobs_aSubstitutionConsumesNothingOfTheShells(t *testing.T) {
	status, stdout, _ := runSetScript(t, "true &\nwait %1 2>/dev/null; sleep 0.1\nfalse &\n"+
		"n=0\nuntil case $(jobs) in *Done*) true;; *) false;; esac || [ $n -ge 200 ]; do n=$((n+1)); sleep 0.05; done\n"+
		"jobs\n")
	// %1 again: the first job was reaped by its wait, and the second took its number.
	if status != 0 || stdout != jobsLine("[1]+  Done(1)") {
		t.Fatalf("status %d stdout %q", status, stdout)
	}
}
