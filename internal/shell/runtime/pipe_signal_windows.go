//go:build windows

package runtime

// setProcessPipe has nothing to change on Windows, which has no SIGPIPE: a write into a pipe
// no one reads fails, for the shell and for the programs it starts alike, and pipe_stage.go
// is all there is of the signal.
func setProcessPipe(pipeDisposition) {}
