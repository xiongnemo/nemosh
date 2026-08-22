package applets

import (
	"io"
	"io/fs"
)

// findExpression is the parsed form of everything after the paths: a tree, and
// the two global options that bound the walk rather than filter it.
//
// It was a flat list of predicates joined by an implicit AND, which is the only
// operator POSIX requires. That made `find` a single-predicate filter rather
// than find: `-name a -o -name b` is a day-one idiom, and `!` was not merely
// missing but read as a *path operand*, so `find . ! -name x` answered
// `find: !: No such file or directory` and blamed a file for an operator.
type findExpression struct {
	root findNode
	// minDepth and maxDepth are the -mindepth/-maxdepth global options.
	// maxDepth is -1 when unbounded. They are not predicates: they bound the
	// traversal, so -maxdepth 1 must stop the walk from *reading* a
	// subdirectory rather than filter its entries out afterwards.
	minDepth int
	maxDepth int
}

// findNode is one node of the expression tree. It reports whether the candidate
// satisfied it, and an action node writes on the way through.
type findNode interface {
	eval(candidate findCandidate, run *findRun) bool
}

// findCandidate is one entry the walk has reached. Predicates that need more
// than a name take it from here rather than re-resolving a path.
type findCandidate struct {
	// display is the path as find prints it: the operand as spelled, then the
	// rest. -path matches against this, and an action writes it.
	display string
	// host is the native path, empty for a synthetic device entry. -empty needs
	// it to read a directory.
	host  string
	entry fs.DirEntry
	depth int
}

func (c findCandidate) info() (fs.FileInfo, error) {
	if c.entry == nil {
		return nil, fs.ErrInvalid
	}
	return c.entry.Info()
}

// findRun carries what an action needs across one walk. The first write error is
// kept rather than returned through every eval signature, and checked by the
// caller once per entry -- an expression is a predicate tree, and threading an
// error return through AND and OR would make short-circuiting mean two things.
type findRun struct {
	stdout io.Writer
	err    error
}

// evaluate applies the expression to one entry, and reports whether the walk
// should keep going.
func (e findExpression) evaluate(candidate findCandidate, run *findRun) error {
	if candidate.depth < e.minDepth {
		return nil
	}
	e.root.eval(candidate, run)
	return run.err
}

// prunes reports whether the walk should stop descending here, which is the
// whole value of -maxdepth: `find . -maxdepth 1` must not read a subdirectory
// only to discard what it finds.
func (e findExpression) prunes(candidate findCandidate) bool {
	return e.maxDepth >= 0 && candidate.depth >= e.maxDepth
}

// Go's && and || short-circuit, which is exactly what find specifies for -a and
// -o: the right side is not evaluated when the left settles the answer. That
// matters for more than speed once an action can appear on either side.
type findAnd struct{ left, right findNode }

func (n findAnd) eval(c findCandidate, run *findRun) bool {
	return n.left.eval(c, run) && n.right.eval(c, run)
}

type findOr struct{ left, right findNode }

func (n findOr) eval(c findCandidate, run *findRun) bool {
	return n.left.eval(c, run) || n.right.eval(c, run)
}

type findNot struct{ inner findNode }

func (n findNot) eval(c findCandidate, run *findRun) bool { return !n.inner.eval(c, run) }

// findTrue is what a global option evaluates to. -maxdepth is an option rather
// than a test, so `find . -maxdepth 1` must still print, and it does so by
// contributing a true term to the implicit AND.
type findTrue struct{}

func (findTrue) eval(findCandidate, *findRun) bool { return true }

// findPrint is the default action and the only one implemented. It answers true
// so that `-print -a -name x` behaves, and it stops writing once the stream has
// failed rather than reporting the same broken pipe once per entry.
type findPrint struct{ terminator byte }

func (n findPrint) eval(c findCandidate, run *findRun) bool {
	if run.err != nil {
		return true
	}
	if _, err := run.stdout.Write(append([]byte(c.display), n.terminator)); err != nil {
		run.err = err
	}
	return true
}
