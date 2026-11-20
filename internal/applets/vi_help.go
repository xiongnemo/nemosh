package applets

// viFeatures is `vi -H`, as busybox's -H lists what its build has: here, what this vi has so
// far, so a key that is missing is a known one.
const viFeatures = `vi: busybox-w32's vi, with vim's ways where the two differ; this build has

	counts        a number before a command, 0 among its digits once one has begun
	moving        h j k l, the arrows, Backspace and Space, ^J ^N ^P, Enter + -
	in a line     0 ^ $ | Home End, f F t T ; ,
	by words      w b e W B E, { } by paragraphs, % to a bracket's pair
	to a line     G gg, H M L on the screen
	the screen    ^F ^B ^D ^U ^E ^Y, Page Up and Down, z. z- z<Enter>, ^G ^L
	inserting     i a I A, the Insert key; Escape or ^C to leave; Enter, Backspace, the arrows
	writing       :w :w! :w FILE :wq :x ZZ
	quitting      :q :q! ZQ
	starting      vi [-R] [+LINE | +] [FILE]

Anything else says it is not implemented. A file whose every line ends in CRLF is written
back with CRLF; a new one with LF.
`
