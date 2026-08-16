//go:build windows

package runtime

import (
	"os"
	"path/filepath"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

// loginDefaults is what this platform can answer when the environment did not.
//
// The value is a *native* path with forward slashes -- `C:/Users/nemo` -- and
// not this shell's own `/c/Users/nemo`. That was the first attempt, on the
// reasoning that `echo $HOME` should agree with `pwd`, and it was wrong for a
// reason worth writing down: HOME is exported, and the programs it is exported
// to are native. Measured:
//
//	$ HOME=/c/Users/nemo   busybox ash -c 'cd $HOME; pwd'  ->  C:/c/Users/nemo
//	$ HOME=C:/Users/nemo   busybox ash -c 'cd $HOME; pwd'  ->  C:/Users/nemo
//
// busybox read the leading slash as "absolute on the current drive" and glued
// the drive on in front, which is what any native program does with it. Git Bash
// gets away with `/c/...` only because MSYS2 rewrites paths in the environment
// as it spawns a native child; this shell has no such layer, and adding one is a
// much larger promise than a default value should be making.
//
// So this follows busybox-w32 exactly, which sets HOME from
// GetUserProfileDirectory and then runs bs_to_slash over it (win32/mingw.c,
// gethomedir). Forward slashes because every Windows API accepts them and
// because a backslash is an escape character in a shell.
//
// The cost is that `echo $HOME` and `pwd` spell the same directory differently.
// This shell accepts both spellings as input, so nothing breaks; it is a wart,
// and it is the smaller one.
func loginDefaults(pathmodel.Model) map[string]string {
	defaults := map[string]string{}
	if profile, err := os.UserHomeDir(); err == nil && profile != "" {
		defaults["HOME"] = filepath.ToSlash(profile)
	}
	if name := applets.CurrentUserName(); name != "" {
		defaults["USER"] = name
		defaults["LOGNAME"] = name
	}
	if executable, err := os.Executable(); err == nil {
		defaults["SHELL"] = filepath.ToSlash(executable)
	}
	return defaults
}
