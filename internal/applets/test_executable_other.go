//go:build !windows

package applets

import "os"

func isExecutableFile(_ string, info os.FileInfo) bool {
	return info.Mode().Perm()&0o111 != 0
}
