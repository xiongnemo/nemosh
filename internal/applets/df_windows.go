package applets

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// Where df's numbers come from on Windows.
//
// **GetDiskFreeSpaceEx, not GetDiskFreeSpace.** The first answers bytes and honours a quota;
// the second answers clusters and does not, so on a volume with a per-user quota the cluster
// form reports the disk's free space rather than the caller's. That difference is the whole
// reason `df` is being asked.
//
// A drive that is not ready -- an empty card reader, a disconnected network letter -- is
// **skipped rather than reported as zero**, because a row of zeroes reads like a full disk.

// collectFilesystems answers a row per operand, or a row per ready volume when there are
// none.
func collectFilesystems(view ProcessView, operands []string, stderr io.Writer) ([]filesystemUsage, error) {
	if len(operands) > 0 {
		return filesystemsForOperands(view, operands, stderr)
	}
	var rows []filesystemUsage
	for _, root := range windowsVolumeRoots() {
		usage, ok := volumeUsage(root)
		if !ok {
			// Not ready. Nothing truthful can be said about it, so nothing is.
			continue
		}
		rows = append(rows, usage)
	}
	return rows, nil
}

func filesystemsForOperands(view ProcessView, operands []string, stderr io.Writer) ([]filesystemUsage, error) {
	var rows []filesystemUsage
	failed := false
	for _, operand := range operands {
		native, err := resolveHostPath(view, operand)
		if err != nil {
			return nil, err
		}
		// busybox's find_mount_point stats FILE first, and what is not there is on no
		// volume. Its letter's was answered, with status 0.
		if _, err := os.Stat(native); err != nil {
			fmt.Fprintf(stderr, "df: %s: cannot find mount point\n", operand)
			failed = true
			continue
		}
		volume := filepath.VolumeName(native)
		if volume == "" {
			fmt.Fprintf(stderr, "df: %s: cannot say which volume this is on\n", operand)
			failed = true
			continue
		}
		usage, ok := volumeUsage(volume + `\`)
		if !ok {
			fmt.Fprintf(stderr, "df: %s: %s is not ready\n", operand, volume)
			failed = true
			continue
		}
		rows = append(rows, usage)
	}
	if failed {
		return rows, ExitStatus(1)
	}
	return rows, nil
}

// windowsVolumeRoots lists the drive letters that exist, as `C:\` and so on.
func windowsVolumeRoots() []string {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	var roots []string
	for letter := 0; letter < 26; letter++ {
		if mask&(1<<uint(letter)) == 0 {
			continue
		}
		roots = append(roots, string(rune('A'+letter))+`:\`)
	}
	return roots
}

// volumeUsage asks Windows for one volume's numbers, and reports whether it could answer.
func volumeUsage(root string) (filesystemUsage, bool) {
	path, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return filesystemUsage{}, false
	}
	// free is what this caller may use, total is the volume's size for this caller, and
	// totalFree is the disk's -- the first two are the ones a quota changes.
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(path, &free, &total, &totalFree); err != nil {
		return filesystemUsage{}, false
	}
	letter := strings.TrimSuffix(root, `\`)
	return filesystemUsage{
		device:    letter,
		mount:     letter + "/",
		fsType:    volumeFilesystem(path),
		used:      total - free,
		available: free,
	}, true
}

// volumeFilesystem is the name Windows gives a volume's filesystem, NTFS, FAT32 or exFAT, as
// busybox-w32's mount table carries it; "-" where it will not say.
func volumeFilesystem(root *uint16) string {
	name := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumeInformation(root, nil, 0, nil, nil, nil, &name[0], uint32(len(name))); err != nil {
		return "-"
	}
	return windows.UTF16ToString(name)
}
