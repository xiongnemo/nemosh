//go:build !windows

package runtime

import "os/exec"

// applyRawCommandLine is unreachable off Windows: only a batch launch needs a
// raw command line, and externalCommand gates that on GOOS. It exists so the
// batch helpers stay compilable — and unit-testable — on every platform.
func applyRawCommandLine(*exec.Cmd, string) {}

// launchWorkingDirectory has nothing to adapt off Windows: only CreateProcess
// bounds a child's working directory. childWorkingDirectory stays in a shared
// file for the same reason the batch helpers do -- so it can be unit-tested
// everywhere rather than only on the platform that needs it.
func launchWorkingDirectory(dir string) (string, error) { return dir, nil }
