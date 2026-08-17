package main

import (
	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// completionDirectory is the shell's working directory in the form the host can
// actually read.
//
// Runtime.WorkingDirectory answers in the shell's own view -- on Windows,
// `/c/Users/nemo/...` -- which is the right answer for a prompt and the wrong
// one for os.ReadDir, which cannot open it. Handing that straight to completion
// meant every file completion in a real session read nothing and offered
// nothing, from the first prompt onwards, however correct the rest of the
// machinery was.
//
// It went unseen because every test built the editor with a native path from
// t.TempDir, so the two forms never met. The lesson is the ordinary one: a seam
// between two path vocabularies has to be crossed somewhere explicit, and the
// place it was being crossed was nowhere.
func completionDirectory(rt runtime.Runtime) string {
	resolved, err := applets.ResolveProcessPath(rt, ".")
	if err != nil || resolved.Device || resolved.Native == "" {
		// Better to complete against the process directory than against nothing:
		// a wrong list is visible and recoverable, an empty one looks like a
		// broken key.
		return currentWorkingDirectory()
	}
	return resolved.Native
}

// nativePath crosses the same seam for a path the shell already holds, and
// reports whether it could.
//
// The seam moved when the launch boundary started translating: HOME is now
// `/c/Users/nemo` inside the shell, so Go code that opens it -- the history
// file, the ssh config -- has to ask for the native spelling rather than reuse
// the string. That is the lesson from completionDirectory above, arriving a
// second time in a different place.
func nativePath(rt runtime.Runtime, path string) (string, bool) {
	if path == "" {
		return "", false
	}
	resolved, err := applets.ResolveProcessPath(rt, path)
	if err != nil || resolved.Device || resolved.Native == "" {
		return "", false
	}
	return resolved.Native, true
}
