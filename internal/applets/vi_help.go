package applets

// viFeatures is `vi -H`, as busybox's -H lists what its build has: here, what this vi has so
// far, so a key that is missing is a known one.
const viFeatures = `vi: busybox-w32's vi, with vim's ways where the two differ; this build has

	moving        h j k l, the arrows, Backspace and Space, 0 $ Home End, ^J ^N ^P
	inserting     i a, the Insert key; Escape or ^C to leave; Enter, Backspace, the arrows
	writing       :w :w! :w FILE :wq :x ZZ
	quitting      :q :q! ZQ
	the screen    ^G ^L
	starting      vi [-R] [+LINE | +] [FILE]

Anything else says it is not implemented. A file whose every line ends in CRLF is written
back with CRLF; a new one with LF.
`
