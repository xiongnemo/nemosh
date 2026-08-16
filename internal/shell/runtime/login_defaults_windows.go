//go:build windows

package runtime

import (
	"os"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

// loginDefaults is what this platform can answer when the environment did not.
//
// HOME is given in the shell's own spelling rather than the host's, so that
// `cd ~; pwd` and `echo $HOME` agree, and so `$HOME` can be compared with `$PWD`
// in a script. Git Bash sets it the same way, which is also what a reader coming
// from there expects to see.
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
