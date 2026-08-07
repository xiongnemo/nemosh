//go:build windows

package runtime

import (
	"strings"
	"syscall"
)

// realCaseNativePath asks the filesystem how each component of an absolute
// native path is actually spelled, so a shell-generated display path shows the
// name on disk rather than the one the user typed. `cd /c/users/NEMO` then
// `pwd` should answer /c/Users/nemo.
//
// docs/design/windows-path-model.md asks for exactly this, "silently preserve
// spelling on failure", which is why every error path returns the input
// unchanged: a directory that has since been removed, a volume that does not
// support the query, and a path that was never absolute all leave the spelling
// alone rather than becoming a diagnostic about something the user did not ask
// for.
//
// FindFirstFile per component rather than GetLongPathName: the latter is
// documented to expand 8.3 names, and correcting case is only incidental to
// that. Measured on Windows 10 19045, `C:\users\nemo\documents` comes back from
// GetLongPathName with `documents` still lowercase while the directory on disk
// is `Documents`; FindFirstFile reports the stored name every time.
func realCaseNativePath(native string) string {
	volume, rest, ok := splitNativeVolume(native)
	if !ok {
		return native
	}
	if rest == "" {
		return volume + `\`
	}
	built := volume
	for _, component := range strings.Split(rest, `\`) {
		actual, asked := storedComponentName(built+`\`+component, component)
		if !asked {
			// Nothing below this point can be asked about either, so the rest
			// keeps the spelling it arrived with.
			return built + `\` + strings.Join(remainingComponents(rest, component), `\`)
		}
		built += `\` + actual
	}
	return built
}

// splitNativeVolume separates `C:` or `\\host\share` from the rest. Anything
// else -- a relative path, a device path -- is not something to ask the
// filesystem about component by component.
func splitNativeVolume(native string) (string, string, bool) {
	normalized := strings.ReplaceAll(native, "/", `\`)
	if strings.HasPrefix(normalized, `\\`) {
		parts := strings.SplitN(strings.TrimPrefix(normalized, `\\`), `\`, 3)
		if len(parts) < 2 {
			return "", "", false
		}
		share := `\\` + parts[0] + `\` + parts[1]
		if len(parts) == 2 {
			return share, "", true
		}
		return share, strings.Trim(parts[2], `\`), true
	}
	if len(normalized) < 2 || normalized[1] != ':' {
		return "", "", false
	}
	return strings.ToUpper(normalized[:1]) + ":", strings.Trim(normalized[2:], `\`), true
}

// storedComponentName answers what the filesystem calls one component. The
// second result says only whether the question could be asked; a component that
// could be asked about but should not be replaced comes back as the fallback
// with asked=true, so the walk carries on into the components below it.
//
// Only a difference of case is adopted. An 8.3 short name such as `RUNNER~1`
// resolves to `runneradmin`, which is a *different* name rather than a
// differently-cased one, and adopting it would rewrite the path the user typed
// instead of correcting it. windows-path-model.md asks for the real case and
// says to preserve the spelling otherwise, and this is the line between the two.
// It is not a hypothetical: GitHub's Windows runners hand out a TEMP under an
// 8.3 alias, because the profile directory name is longer than eight
// characters.
func storedComponentName(path, fallback string) (string, bool) {
	// The extended-length prefix keeps the query off the MAX_PATH ceiling,
	// which matters because Nemosh's working directory is not bound by it.
	query, err := syscall.UTF16PtrFromString(extendedLengthPath(path))
	if err != nil {
		return fallback, false
	}
	var data syscall.Win32finddata
	handle, err := syscall.FindFirstFile(query, &data)
	if err != nil {
		return fallback, false
	}
	_ = syscall.FindClose(handle)
	name := syscall.UTF16ToString(data.FileName[:])
	if name == "" || !strings.EqualFold(name, fallback) {
		return fallback, true
	}
	return name, true
}

// remainingComponents returns the components from the failed one onwards, so
// the untouched tail can be joined back on.
func remainingComponents(rest, from string) []string {
	components := strings.Split(rest, `\`)
	for index, component := range components {
		if component == from {
			return components[index:]
		}
	}
	return components
}
