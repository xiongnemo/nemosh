package runtime

import (
	"os"
	"strings"
)

// bashDotDotMissing reports, in bash mode, a directory before a `..` in target that is not
// there: bash's cd checks each one before it takes the `..` away -- sh_canonpath with
// PATH_CHECKDOTDOT -- so `cd nonexistent/..` is `No such file or directory`, where busybox takes
// the `..` away first and goes to where the path ends, as nemosh and sh do. Windows itself takes
// a `..` away before it looks, so the physical cd, -P, would go there too; bash's, through MSYS,
// does not, and is refused the same way. Measured against Git for Windows' bash 5.3.
func (r Runtime) bashDotDotMissing(target string) bool {
	if !r.BashMode() || !strings.Contains(target, "..") {
		return false
	}
	components := strings.Split(target, "/")
	for index, component := range components {
		if component != ".." || index == 0 {
			continue
		}
		before := strings.Join(components[:index], "/")
		if before == "" {
			// `/..` is the root, which is there.
			continue
		}
		resolved, err := r.ResolveNemoshPath(before)
		if err != nil || resolved.Device {
			continue
		}
		if info, err := os.Stat(resolved.Native); err != nil || !info.IsDir() {
			return true
		}
	}
	return false
}
