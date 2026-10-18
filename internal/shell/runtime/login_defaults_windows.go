//go:build windows

package runtime

import (
	"os"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// loginDefaults is what this platform can answer when the environment did not.
//
// The value is in this shell's own spelling -- `C:/Users/nemo`, or `/c/Users/nemo` under
// NEMOSH_PATHS=msys -- like every other path it reports, so `echo $HOME`, `echo ~` and
// `pwd` all say the same thing about the same directory.
//
// It briefly was not. Setting it in the alias spelling and exporting it verbatim broke
// every native program launched from here:
//
//	$ HOME=/c/Users/nemo busybox ash -c 'cd $HOME; pwd'  ->  C:/c/Users/nemo
//
// The answer then was not to pick the other spelling and live with $HOME disagreeing
// with pwd -- it was to translate at the one place the two worlds meet, which
// environment_launch.go still does. That covers the variables and not the words: `~` in
// `nvim ~/.ssh/config` is expanded before nvim is run, so in the alias spelling nvim was
// handed `/c/Users/...` and read it as `\c\Users\...` on the current drive. Hence the
// spelling busybox-w32 uses, which every Windows program reads.
//
// The directory itself is busybox-w32's: GetUserProfileDirectory rather than
// %USERPROFILE%, which os.UserHomeDir reads (win32/mingw.c, gethomedir).
func loginDefaults(paths pathState) map[string]string {
	defaults := map[string]string{}
	if profile, err := os.UserHomeDir(); err == nil && profile != "" {
		if canonical, err := paths.model.Resolve(profile); err == nil {
			defaults["HOME"] = paths.spell(canonical)
		}
	}
	if name := applets.CurrentUserName(); name != "" {
		defaults["USER"] = name
		defaults["LOGNAME"] = name
	}
	if executable, err := os.Executable(); err == nil {
		if canonical, err := paths.model.Resolve(executable); err == nil {
			defaults["SHELL"] = paths.spell(canonical)
		}
	}
	return defaults
}
