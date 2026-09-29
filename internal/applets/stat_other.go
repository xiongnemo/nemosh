//go:build !windows

package applets

import (
	"os"
	"os/user"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)

// hostStatRecord is the host's own lstat, or its stat with follow, which is all busybox asks.
func hostStatRecord(native string, follow bool, _ uint32) (statRecord, error) {
	var st unix.Stat_t
	stat := unix.Lstat
	if follow {
		stat = unix.Stat
	}
	if err := stat(native, &st); err != nil {
		return statRecord{}, &os.PathError{Op: "stat", Path: native, Err: err}
	}
	record := statRecord{
		mode: uint32(st.Mode), size: st.Size, blocks: st.Blocks, blockSize: int64(st.Blksize),
		device: uint64(st.Dev), inode: uint64(st.Ino), links: uint64(st.Nlink), uid: st.Uid, gid: st.Gid,
		major: unix.Major(uint64(st.Rdev)), minor: unix.Minor(uint64(st.Rdev)),
		access: time.Unix(st.Atim.Unix()), modify: time.Unix(st.Mtim.Unix()), change: time.Unix(st.Ctim.Unix()),
	}
	record.owner, record.group = userName(st.Uid), groupName(st.Gid)
	if record.mode&statTypeMask == statLink {
		record.target, _ = os.Readlink(native)
	}
	return record, nil
}

// userName and groupName are what getpwuid and getgrgid name an id, and UNKNOWN, busybox's word,
// when they name nothing.
func userName(id uint32) string {
	if account, err := user.LookupId(strconv.FormatUint(uint64(id), 10)); err == nil {
		return account.Username
	}
	return "UNKNOWN"
}

func groupName(id uint32) string {
	if group, err := user.LookupGroupId(strconv.FormatUint(uint64(id), 10)); err == nil {
		return group.Name
	}
	return "UNKNOWN"
}

// currentStatOwner is the uid, gid and their names of the account this runs as.
func currentStatOwner() (uint32, uint32, string, string) {
	uid, gid := uint32(os.Getuid()), uint32(os.Getgid())
	return uid, gid, userName(uid), groupName(gid)
}

// hostStatfs is the host's own statfs.
func hostStatfs(native string) (statfsRecord, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(native, &st); err != nil {
		return statfsRecord{}, &os.PathError{Op: "statfs", Path: native, Err: err}
	}
	return statfsRecord{
		id:         uint64(uint32(st.Fsid.Val[0]))<<32 | uint64(uint32(st.Fsid.Val[1])),
		nameLength: statfsNameLength(&st), kind: uint64(st.Type), blockSize: uint64(st.Bsize),
		blocks: st.Blocks, free: st.Bfree, available: st.Bavail, files: st.Files, freeFiles: st.Ffree,
	}, nil
}
