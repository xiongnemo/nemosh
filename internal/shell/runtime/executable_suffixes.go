package runtime

import "runtime"

// ExecutableSuffixes are the suffixes a bare name is tried with, in order: on Windows `git`
// runs git.exe and `deploy` runs deploy.sh, and elsewhere a name runs only as it is spelled.
// Exported so that completion offers a name by exactly the suffixes it runs by; the two lists
// were kept apart, and `.sh` was in one and not the other.
func ExecutableSuffixes() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	return append([]string(nil), windowsExecutableSuffixes[:]...)
}
