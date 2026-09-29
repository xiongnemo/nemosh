package runtime

// initialFileModeMask is where a shell's umask starts on Windows, which gives a process none to
// inherit: 0022, as Git for Windows's bash starts, and as the Oils cases that print a new
// shell's umask expect. busybox-w32 starts at its DEFAULT_UMASK, 0002 (include/mingw.h:363),
// which its stat and ls show as files 0664 and directories 0775; taking that fails those two
// cases, and is left for a decision rather than taken.
const initialFileModeMask uint16 = 0o022
