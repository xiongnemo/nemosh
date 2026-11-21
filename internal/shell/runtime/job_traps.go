package runtime

// jobTraps are the traps a background job starts with, under either launcher, and the ones it
// lists without running them.
//
// What the shell ignores -- a trap whose action is empty -- stays ignored in the job, as POSIX
// has a subshell keep it: the job lists it, as both references' do, and a TERM `kill` sends it
// is ignored, as bash's is. busybox-w32 ends the job whatever it has, having no way to deliver a
// signal to one; see signal_job_test.go. A trap that runs something is the shell's, and the job
// starts without it, as in both. The job had none at all, so ignoring TERM sheltered no job.
//
// In bash mode the job lists the shell's other traps, as bash's subshell lists them (see
// trap_listed.go), and under `set -E` runs its ERR trap, for the commands inside the job but not
// for the job's own failure (jobTop): `{ false; } &` runs it, `false &` does not. busybox's job
// lists only what it ignores and runs no ERR trap, as nemosh and sh go on doing.
func (r Runtime) jobTraps() (traps, listed map[trapName]string) {
	traps = map[trapName]string{}
	for name, action := range r.traps {
		if action == "" {
			traps[name] = ""
		}
	}
	if !r.BashMode() {
		return traps, nil
	}
	if action, set := r.traps[trapERR]; set && r.options.errTrace {
		traps[trapERR] = action
	}
	return traps, r.droppedTraps(traps)
}
