package applets

import "io"

// sedOutput writes sed's lines, holding each newline back until it knows whether
// anything follows.
//
// The rule both references implement is not "each line keeps the ending it had".
// It is narrower: **the newline is omitted only on the very last thing written,
// and only when the input's final line had none.** Measured, on a three-byte file
// holding `a\nb`:
//
//	sed s/x/y/  ->  a\nb          three bytes, the b bare
//	sed p       ->  a\na\nb\nb    the duplicated b *does* get a newline
//
// So `p` writes `b` twice and the first of them is terminated. A per-line rule
// cannot express that, because the same line is written twice with two different
// endings; what distinguishes them is only that one is last.
//
// Hence the deferral. Every write owes a newline, paid before the *next* write,
// and the final debt is forgiven if the input ended without one. sed previously
// used Fprintln throughout, which added a newline the input did not have -- and
// with `-i` wrote that byte to the file, so `sed -i` on a file with no final
// newline grew it even when the script matched nothing.
type sedOutput struct {
	out io.Writer
	// owes records that something has been written and its newline is not out yet.
	owes bool
	// sourceEnded is whether the input line that produced the most recent write was
	// terminated. It decides the final newline, and it belongs to the *write*
	// rather than to the stream -- see close.
	sourceEnded bool
}

func newSedOutput(out io.Writer) *sedOutput { return &sedOutput{out: out} }

// writeLine writes one line, paying for the previous one first.
//
// sourceEnded travels with the write because the last thing written is not always
// produced by the last line read. `sed 2d` on a two-line file whose second line has
// no terminator deletes that second line, so the last output came from the *first* --
// which was terminated -- and both references answer with the newline. Asking the
// stream at the end would have asked about the wrong line.
func (o *sedOutput) writeLine(text string, sourceEnded bool) error {
	if o.owes {
		if _, err := io.WriteString(o.out, "\n"); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(o.out, text); err != nil {
		return err
	}
	o.owes, o.sourceEnded = true, sourceEnded
	return nil
}

// close settles the last newline, or does not.
//
// Nothing was written at all when owes is false, and then there is no newline to
// argue about.
//
// One case where the two references disagree, and busybox is followed because it is
// the primary reference here and because its answer is the more principled one: on
// a two-line file with no final terminator, `sed 2q` gives the file back unchanged
// from busybox and adds a byte under GNU. Adding a byte to a file that did not have
// one is the behaviour this whole change exists to remove.
func (o *sedOutput) close() error {
	if !o.owes || !o.sourceEnded {
		return nil
	}
	_, err := io.WriteString(o.out, "\n")
	return err
}
