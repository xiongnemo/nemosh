package pathmodel

// Where each drive was left, which is what a drive-relative path is relative to.
//
// Windows keeps a current directory for every drive, not only the one a process is on, and
// `D:` names the one D: was left at: cmd's `cd /d D:\BT` then `cd /d C:\Windows` then `dir D:`
// lists D:\BT. busybox-w32 keeps them by changing its own process's directory, so it has them
// for nothing, and measured with FRP-6354, after `cd D:/BT; cd C:/Windows`:
//
//	cd D:          D:/BT
//	cd C:          C:/Windows, the drive it is on
//	cd D:..        D:/
//	ls -d C:System32, cat C:file   relative to C:'s directory
//
// and a drive it has not been to is at its root. This shell never changes its process's
// directory -- a pipeline stage and a subshell each have their own -- so it keeps the table
// itself, and refused the form instead: `cd D:` said "drive-relative Windows paths are not
// supported", in a shell for the platform where it is how a drive is changed.

// driveDirectories is the directory each drive was last at, by letter, a to z. A change makes
// a new table, so one shared between copies of a model is never written.
type driveDirectories [26]Path

// with is the table with path recorded for its drive, the same table when it is already
// recorded, and itself for a path on no drive -- a UNC share or a virtual root.
func (d *driveDirectories) with(path Path) *driveDirectories {
	drive, _, ok := driveShort(string(path))
	if !ok {
		return d
	}
	index := drive[0] - 'a'
	if d != nil && d[index] == path {
		return d
	}
	updated := new(driveDirectories)
	if d != nil {
		*updated = *d
	}
	updated[index] = path
	return updated
}

// driveDirectory is the directory a drive-relative path on drive starts from: the current one
// for the drive this model is on, where the drive was left for another, and its root for a
// drive not yet been to.
func (m Model) driveDirectory(drive string) Path {
	if m.root == Path("/"+drive) {
		return m.cwd
	}
	if m.drives != nil && m.drives[drive[0]-'a'] != "" {
		return m.drives[drive[0]-'a']
	}
	return Path("/" + drive)
}

// DriveDirectories is where each drive was left, by lower-case letter: what a launched program
// is handed as its `=D:` variables, so that `D:` means to it what it means here, as it does to a
// program busybox-w32 starts.
func (m Model) DriveDirectories() map[string]Path {
	if m.drives == nil {
		return nil
	}
	drives := map[string]Path{}
	for index, path := range m.drives {
		if path != "" {
			drives[string(rune('a'+index))] = path
		}
	}
	return drives
}
