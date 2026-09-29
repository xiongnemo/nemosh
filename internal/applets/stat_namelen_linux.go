package applets

import "golang.org/x/sys/unix"

// statfsNameLength is f_namelen, the longest name the filesystem holds.
func statfsNameLength(st *unix.Statfs_t) uint64 { return uint64(st.Namelen) }
