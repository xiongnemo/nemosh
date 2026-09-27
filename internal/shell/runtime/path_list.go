package runtime

import goruntime "runtime"

// On Windows, PATH, CDPATH and MANPATH take a `:` between their directories as well as the
// `;` Windows programs read, as busybox-w32 has them (fix_pathvar in its shell/ash.c): an
// assignment rewrites each `:` that is not a drive letter's into `;`. So `PATH=bin:$PATH`,
// which is how a script written for Unix puts a directory first, finds what is in bin, and a
// program it starts is handed a list it can read. Here it was one directory, named
// `bin:C:\Users...`, which held nothing.
func windowsPathList(name, value string) string {
	if goruntime.GOOS != "windows" {
		return value
	}
	switch name {
	case "PATH", "CDPATH", "MANPATH":
	default:
		return value
	}
	path := []byte(value)
	for index := 0; index < len(path); {
		if path[index] != ':' && path[index] != ';' {
			// A drive letter's colon is part of the directory, at the start of one.
			if isASCIILetter(rune(path[index])) && index+1 < len(path) && path[index+1] == ':' {
				index += 2
			}
			for index < len(path) && path[index] != ':' && path[index] != ';' {
				index++
			}
		}
		if index < len(path) {
			if path[index] == ':' {
				path[index] = ';'
			}
			index++
		}
	}
	return string(path)
}
