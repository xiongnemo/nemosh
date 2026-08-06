//go:build !windows

package runtime

import "os/exec"

// applyRawCommandLine is unreachable off Windows: only a batch launch needs a
// raw command line, and externalCommand gates that on GOOS. It exists so the
// batch helpers stay compilable — and unit-testable — on every platform.
func applyRawCommandLine(*exec.Cmd, string) {}
