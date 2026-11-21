package runtime

// jobTraps are the traps a background job starts with, under either launcher: what the shell
// ignores -- a trap whose action is empty -- which stays ignored in the job, as POSIX has a
// subshell keep it: the job lists it, as both references' do, and a TERM `kill` sends it is
// ignored, as bash's is. busybox-w32 ends the job whatever it has, having no way to deliver a
// signal to one; see signal_job_test.go. A trap that runs something is the shell's, and the job
// starts without it, as in both. The job had none at all, so ignoring TERM sheltered no job.
func (r Runtime) jobTraps() map[trapName]string {
	traps := map[trapName]string{}
	for name, action := range r.traps {
		if action == "" {
			traps[name] = ""
		}
	}
	return traps
}
