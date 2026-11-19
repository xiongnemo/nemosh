package applets

import "io"

// sedOutput writes sed's lines as busybox's puts_maybe_newline does (editors/sed.c): a line
// and its newline together, unless the line is written with the ending of an input line that
// had none. That newline is owed, and paid before the next thing written to the same output;
// if nothing is, the output ends without it, as the input did.
//
// Which ending a write carries is the command's. The pattern space printed at the end of the
// cycle, and by n, q, s///p, s///w and w, carries its input line's; p, P, =, i, c, a and r's
// lines always end theirs, and G and x make the pattern space an ended line (sed_hold.go).
// Measured on busybox, on a three-byte file holding `a\nb`:
//
//	sed s/x/y/  ->  a\nb          the b bare, as it came
//	sed p       ->  a\na\nb\nb    p's b ended, the printed one bare
//	sed -n p    ->  a\nb\n        p ends it
//	sed s/b//   ->  a\n           an empty line owes nothing
//
// Every newline was held back until the next write, and the last forgiven when the input's
// last line had none. The pattern space came out the same, but p, =, a, c, r, G and x lost
// the newline busybox ends them with, `s/b//p` wrote one too many, and two outputs sharing a
// destination, as `w /dev/stdout` shares the standard output's, would put two lines on one.
//
// One place busybox is not followed: its = prints with fprintf past the owed newline, so
// after a FILE whose last line had none, `sed = f1 f2` runs that line into the next number.
//
// separator is what ends the pattern space, a newline, or under -z a NUL, as GNU's
// buffer_delimiter; what is not the pattern space -- =, a, i, c and l -- ends with a newline
// whichever, as GNU's does.
type sedOutput struct {
	out io.Writer
	// owes is that the last thing written was text with no ending after it, and owed the
	// ending it would have had.
	owes      bool
	owed      byte
	separator byte
}

func newSedOutput(out io.Writer) *sedOutput { return &sedOutput{out: out, separator: '\n'} }

// flush hands on what a buffered output holds, before a write elsewhere that may share its
// destination: `w /dev/stdout`.
func (o *sedOutput) flush() error {
	if buffered, ok := o.out.(interface{ Flush() error }); ok {
		return buffered.Flush()
	}
	return nil
}

// writeLine writes the ending owed, the pattern space's text, and the separator if ended, as
// one write.
func (o *sedOutput) writeLine(text string, ended bool) error {
	return o.writeEnded(text, ended, o.separator)
}

// writeText writes text that is not the pattern space, ended with a newline.
func (o *sedOutput) writeText(text string) error { return o.writeEnded(text, true, '\n') }

func (o *sedOutput) writeEnded(text string, ended bool, ending byte) error {
	line := text
	if o.owes {
		line = string(o.owed) + line
	}
	if ended {
		line += string(ending)
	}
	o.owes, o.owed = !ended && text != "", ending
	if line == "" {
		return nil
	}
	_, err := io.WriteString(o.out, line)
	return err
}
