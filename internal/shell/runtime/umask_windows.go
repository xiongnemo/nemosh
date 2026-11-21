package runtime

import (
	"os"
	"os/exec"
)

// initialFileModeMask is where a shell's umask starts on Windows, which gives a process none to
// inherit: 0002, busybox-w32's DEFAULT_UMASK (include/mingw.h:363), so its stat and ls show
// files 0664 and directories 0775 as busybox's do. It was 0022, as Git for Windows's bash
// starts; the user chose busybox's, and bash mode, which follows bash where the two part,
// starts at 0022 (EnterBashMode), as the two Oils cases that print a new shell's umask expect.
const initialFileModeMask uint16 = 0o002

// startChild starts command as it is: a Windows child has no umask to be given.
func (r Runtime) startChild(command *exec.Cmd) error {
	return command.Start()
}

// createMode is perm as it is. A mode without the owner's write bit makes a read-only file on
// Windows, and what the umask should do here is left for the same decision as where it starts.
func (r Runtime) createMode(perm os.FileMode) os.FileMode {
	return perm
}
