//go:build windows

package runtime

import (
	"os"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

// loginDefaults is what this platform can answer when the environment did not.
//
// The value is in this shell's own spelling -- `/c/Users/nemo` -- like every
// other path it reports, so `echo $HOME`, `echo ~` and `pwd` all say the same
// thing about the same directory.
//
// It briefly was not. Setting it this way and exporting it verbatim broke every
// native program launched from here:
//
//	$ HOME=/c/Users/nemo busybox ash -c 'cd $HOME; pwd'  ->  C:/c/Users/nemo
//
// The answer is not to pick the other spelling and live with $HOME disagreeing
// with pwd -- it is to translate at the one place the two worlds meet. See
// environment_launch.go.
//
// The directory itself is busybox-w32's: GetUserProfileDirectory rather than
// %USERPROFILE%, which os.UserHomeDir reads (win32/mingw.c, gethomedir).
func loginDefaults(model pathmodel.Model) map[string]string {
	defaults := map[string]string{}
	if profile, err := os.UserHomeDir(); err == nil && profile != "" {
		if canonical, err := model.Resolve(profile); err == nil {
			defaults["HOME"] = string(canonical)
		}
	}
	if name := applets.CurrentUserName(); name != "" {
		defaults["USER"] = name
		defaults["LOGNAME"] = name
	}
	if executable, err := os.Executable(); err == nil {
		if canonical, err := model.Resolve(executable); err == nil {
			defaults["SHELL"] = string(canonical)
		}
	}
	return defaults
}
