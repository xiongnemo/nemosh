package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// -exec, -ok and -delete, as busybox's find has them (findutils/find.c). They were refused by
// name, -exec for want of an execution model and -delete for want of a decision about it.
//
// The model is xargs's: CMD is an applet, run in the shell's process with find's standard
// streams, and a name no applet has is said, `find: CMD: No such file or directory`, and is
// false. `-exec CMD ARGS ;` runs CMD once for each entry, every `{}` in ARGS that entry's path,
// and is true when CMD ends 0. `-exec CMD ARGS {} +` gathers the entries, true for each, and
// runs CMD with as many as a command line holds, the word with the `{}` once for each; one
// that ends other than 0 at the end makes find's status 1. -ok is -exec's `;` form that asks
// first, the command line and a `?` on stderr, an answer beginning y on stdin. -delete removes
// an entry -- a directory only when it is empty, and never `.` -- says a failure and is true
// all the same, and walks each directory after its entries, as busybox's does.

// findExecRoom is how many bytes of paths a `+` command line takes, xargs's room.
const findExecRoom = 32*1024 - 2048

// findApplets is the registry -exec runs CMD from, set once the registry exists: the registry
// holds find, so naming it from find's own code would be an initialization cycle.
var findApplets func(string) (Applet, bool)

func init() { findApplets = DefaultRegistry.Lookup }

// findExec is one -exec or -ok.
type findExec struct {
	argv []string
	ok   bool
	// batch is the paths a `+` form has gathered and not yet run with; nil for `;`.
	batch *findBatch
}

type findBatch struct {
	paths []string
	size  int
}

// execPredicate reads -exec's or -ok's CMD and ARGS up to a `;`, or for -exec a `+`.
func (p *findParser) execPredicate(operand string) (findNode, error) {
	node := findExec{ok: operand == "-ok"}
	for {
		if p.index >= len(p.args) {
			return nil, fmt.Errorf("%s requires an argument", operand)
		}
		word := p.next()
		if word == ";" || word == "+" && !node.ok {
			if word == "+" {
				node.batch = &findBatch{}
			}
			break
		}
		node.argv = append(node.argv, word)
	}
	if len(node.argv) == 0 {
		return nil, fmt.Errorf("%s requires an argument", operand)
	}
	if node.batch != nil {
		substitutions := 0
		for _, word := range node.argv {
			substitutions += strings.Count(word, "{}")
		}
		if substitutions != 1 {
			return nil, fmt.Errorf("only one '{}' allowed for -exec +")
		}
		p.expression.batches = append(p.expression.batches, &node)
	}
	p.hasAction = true
	return node, nil
}

func (n findExec) eval(c findCandidate, run *findRun) bool {
	if n.batch == nil {
		return run.execute(n.command(c.display), n.ok) == 0
	}
	n.batch.paths = append(n.batch.paths, c.display)
	n.batch.size += len(c.display) + 1
	if n.batch.size < findExecRoom {
		return true
	}
	return run.flush(n) == 0
}

// command is the command line for one entry: each `{}` in a word its path.
func (n findExec) command(path string) []string {
	argv := make([]string, len(n.argv))
	for index, word := range n.argv {
		argv[index] = strings.ReplaceAll(word, "{}", path)
	}
	return argv
}

// batchCommand is the command line for a `+` form's gathered paths: the word with the `{}`
// once for each of them.
func (n findExec) batchCommand() []string {
	var argv []string
	for _, word := range n.argv {
		if !strings.Contains(word, "{}") {
			argv = append(argv, word)
			continue
		}
		for _, path := range n.batch.paths {
			argv = append(argv, strings.ReplaceAll(word, "{}", path))
		}
	}
	return argv
}

// flush runs a `+` form's gathered paths, and answers the command's status.
func (run *findRun) flush(n findExec) int {
	if len(n.batch.paths) == 0 {
		return 0
	}
	argv := n.batchCommand()
	n.batch.paths, n.batch.size = nil, 0
	return run.execute(argv, false)
}

// finish runs what every `+` form has left once the walk is over, as busybox's
// flush_exec_plus does: one that ends other than 0 makes find's status 1.
func (run *findRun) finish(expression findExpression) {
	for _, node := range expression.batches {
		if run.flush(*node) != 0 {
			run.failed = true
		}
	}
}

// execute runs argv as find runs a command, asking first for -ok, and answers its status.
func (run *findRun) execute(argv []string, ask bool) int {
	if run.err != nil {
		return 1
	}
	if ask {
		fmt.Fprintf(run.stderr, "%s ?", strings.Join(argv, " "))
		if !askYes(run.answers()) {
			return 1
		}
	}
	applet, found := findApplets(argv[0])
	if !found {
		applet, found = programFor(run.ctx, argv[0])
	}
	if !found {
		fmt.Fprintf(run.stderr, "find: %s: No such file or directory\n", argv[0])
		return 127
	}
	return xargsStatus(argv[0], applet.Run(run.ctx, argv[1:], run.stdin, run.stdout, run.stderr), run.stderr)
}

// answers is stdin, read a line at a time for -ok's questions.
func (run *findRun) answers() io.Reader {
	if run.stdin == nil {
		return nil
	}
	if run.lines == nil {
		run.lines = bufio.NewReader(run.stdin)
	}
	return run.lines
}

// findDelete is -delete.
type findDelete struct{}

func (findDelete) eval(c findCandidate, run *findRun) bool {
	if c.host == "" {
		return true
	}
	var err error
	if c.entry != nil && c.entry.IsDir() {
		if c.display == "." {
			return true
		}
		err = removeDirectory(c.host)
	} else {
		err = removeForOverwrite(c.host)
	}
	if err != nil {
		fmt.Fprintf(run.stderr, "find: %s: %s\n", c.display, causeText(err))
	}
	return true
}

// findRunFor is the run state an applet invocation walks with.
func findRunFor(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) *findRun {
	return &findRun{ctx: ctx, stdin: stdin, stdout: stdout, stderr: stderr}
}
