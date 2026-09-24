package runtime

import "os"

// JobsAreProcesses is the launcher this test binary's jobs take, by the rule the shell
// applies to NEMOSH_JOBS (jobsAreProcesses), for the tests outside the package whose
// answers depend on it: `$!` is a pid only when a job is a process.
func JobsAreProcesses() bool {
	return jobsAreProcesses(os.Getenv("NEMOSH_JOBS"))
}
