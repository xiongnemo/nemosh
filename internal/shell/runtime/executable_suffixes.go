package runtime

// ExecutableSuffixes are the suffixes a bare name is tried with, in order: `git` runs git.exe
// and `deploy` runs deploy.sh. Exported so that completion offers a name by exactly the suffixes
// it runs by; the two lists were kept apart, and `.sh` was in one and not the other.
func ExecutableSuffixes() []string {
	return append([]string(nil), windowsExecutableSuffixes[:]...)
}
