//go:build !windows

package runtime

// endJobsWithTheTestBinary has no counterpart here: a job process is in a process group of
// its own, which nothing ends when the test binary does. The runners that run these
// tests off Windows end every process a job left when the job does.
func endJobsWithTheTestBinary() {}
