//go:build !windows

package applets

import (
	"fmt"
	"io"

	"golang.org/x/sys/unix"
)

// df's numbers away from Windows.
//
// **It reports the filesystems the operands are on, and the current directory's when there
// are none** -- it does not enumerate every mount. Walking the mount table is a different
// job on each system that has one (`/proc/mounts` on Linux, `getmntinfo` on the BSDs and
// macOS), and this build exists so the package compiles and its tests run on the CI runners
// rather than to replace the `df` those systems already ship. Saying so is better than a
// third implementation nobody uses.
//
// Recorded in docs/support-matrix.md as a platform difference rather than left to be
// discovered.

func collectFilesystems(view ProcessView, operands []string, stderr io.Writer) ([]filesystemUsage, error) {
	if len(operands) == 0 {
		operands = []string{"."}
	}
	var rows []filesystemUsage
	failed := false
	for _, operand := range operands {
		native, err := resolveHostPath(view, operand)
		if err != nil {
			return nil, err
		}
		var stat unix.Statfs_t
		if err := unix.Statfs(native, &stat); err != nil {
			fmt.Fprintf(stderr, "df: %s: %s\n", operand, CauseText(err))
			failed = true
			continue
		}
		size := uint64(stat.Bsize)
		rows = append(rows, filesystemUsage{
			device:    native,
			mount:     native,
			used:      (stat.Blocks - stat.Bfree) * size,
			available: stat.Bavail * size,
		})
	}
	if failed {
		return rows, ExitStatus(1)
	}
	return rows, nil
}
