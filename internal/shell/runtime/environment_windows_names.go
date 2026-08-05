package runtime

import "strings"

func canonicalWindowsEnvironmentName(name string) (string, bool) {
	canonical := strings.ToUpper(name)
	switch canonical {
	case "APPDATA", "COMSPEC", "LOCALAPPDATA", "PATH", "PATHEXT", "PROGRAMDATA", "SYSTEMROOT", "TEMP", "TMP", "USERPROFILE", "WINDIR":
		return canonical, true
	default:
		return "", false
	}
}

func newHostEnvironment(items []string, platform environmentPlatform) Environment {
	if platform != windowsEnvironment {
		return NewEnvironment(items)
	}
	canonicalInputs := make(map[string]struct{})
	for _, item := range items {
		name, _, found := strings.Cut(item, "=")
		canonical, known := canonicalWindowsEnvironmentName(name)
		if found && known && name == canonical {
			canonicalInputs[canonical] = struct{}{}
		}
	}
	env := NewEnvironment(nil)
	for _, item := range items {
		name, value, found := strings.Cut(item, "=")
		if !found {
			continue
		}
		canonical, known := canonicalWindowsEnvironmentName(name)
		if !known {
			env.Set(name, value)
			continue
		}
		if _, canonicalExists := canonicalInputs[canonical]; canonicalExists && name != canonical {
			continue
		}
		env.Set(canonical, value)
	}
	return env
}
