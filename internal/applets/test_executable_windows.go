package applets

import (
	"os"
	"path/filepath"
	"strings"
)

// Windows has no execute permission bit, so busybox-w32 synthesises one: its
// stat sets S_IXUSR when the name carries an executable suffix or the file
// sniffs as an executable format (win32/mingw.c:780-784), and mingw_access
// then reads that bit for X_OK (win32/mingw.c:2131). `test -x` follows the
// suffix half of that rule; the sniffing half belongs to executable lookup,
// which already does it in internal/shell/runtime/external_format.go.
func isExecutableFile(operand string, info os.FileInfo) bool {
	if info.IsDir() {
		return true
	}
	suffix := strings.ToLower(filepath.Ext(operand))
	switch suffix {
	case ".com", ".exe", ".bat", ".cmd":
		return true
	}
	return false
}
