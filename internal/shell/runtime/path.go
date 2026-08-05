package runtime

import (
	"strings"
)

func platformPath(path string) string {
	if len(path) >= 3 && path[0] == '/' && path[2] == '/' && isDriveLetter(rune(path[1])) {
		return string(path[1]) + ":/" + path[3:]
	}
	return path
}

func (r Runtime) resolvePath(path string) string {
	resolved, err := r.ResolveNemoshPath(path)
	if err != nil {
		return ""
	}
	if resolved.Device {
		return string(resolved.Canonical)
	}
	return resolved.Native
}

func filepathDisplay(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if len(path) >= 2 && path[1] == ':' && isDriveLetter(rune(path[0])) {
		return "/" + strings.ToLower(path[:1]) + path[2:]
	}
	return path
}

func isDriveLetter(r rune) bool {
	return ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
}
