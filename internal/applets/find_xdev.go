package applets

import "slices"

// -xdev keeps the walk on the volumes its PATHs are on, as busybox's has it: a directory on
// another is evaluated as any entry is, and not gone into, as busybox stops at a mount point
// after it has acted on it. The volume is the one busybox-w32's stat reports, the volume serial
// number, so a directory that mounts another volume, or a junction to one, is where it stops.

// findVolumes is each PATH's volume, as busybox stats them before the walk. A PATH that is not
// there has none, and matches no directory.
func findVolumes(view ProcessView, paths []string) []uint64 {
	var volumes []uint64
	for _, path := range paths {
		host, err := resolveHostPath(view, path)
		if err != nil {
			continue
		}
		if record, err := hostStatRecord(host, true, 0); err == nil {
			volumes = append(volumes, record.device)
		}
	}
	return volumes
}

// leavesVolume is whether -xdev keeps the walk out of the directory host: it is on none of the
// PATHs' volumes. One stat cannot answer for is gone into, as before.
func (run *findRun) leavesVolume(expression findExpression, host string) bool {
	if !expression.oneVolume {
		return false
	}
	record, err := hostStatRecord(host, false, 0)
	return err == nil && !slices.Contains(run.volumes, record.device)
}
