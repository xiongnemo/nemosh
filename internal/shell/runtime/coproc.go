package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// coproc is bash's `coproc [NAME] command`, which busybox has not got. The command runs as
// a background job with a pipe to its stdin and one from its stdout. The shell gets the
// other ends as `${NAME[1]}` and `${NAME[0]}`: 60 and 63, the numbers bash picks. NAME is
// COPROC unless the command is a compound one, the only kind bash lets be named.
// `$NAME_PID` is the job's `$!`: its pid when jobs are processes (NEMOSH_JOBS=process),
// `%N` otherwise. A `wait` that reaps it closes both ends and unsets both names, as bash's
// does.
//
// The job is an ordinary one, whichever launcher starts it: a group whose redirections move
// the pipes onto its 0 and 1. Its table holds the job's ends and never the shell's, so
// when the shell closes `${NAME[1]}` the job reads the end of its input.
type coprocNode struct {
	name string
	body programNode
}

func (coprocNode) programNode() {}

// coprocState is what a reaped coprocess leaves to clean up: its names, and its two
// descriptors in the table they were bound in. That table, and not the one `wait` runs
// with, which is a copy taken for the command.
type coprocState struct {
	name          string
	table         *fdTable
	read, written int
}

// coprocLine reads a line that begins with `coproc`: the coprocess, and whatever else the
// line runs after it -- `coproc cat; echo hi >&"${COPROC[1]}"` starts cat and then echoes.
func coprocLine(line string, budget *parseBudget, depth int) ([]programNode, bool, error) {
	rest, ok := strings.CutPrefix(line, "coproc")
	if !ok || rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return nil, false, nil
	}
	name, rest := coprocName(strings.TrimSpace(rest))
	script, err := parseNestedScript(rest, "", budget, depth)
	if err != nil {
		return nil, true, err
	}
	if len(script.program) == 0 {
		return nil, true, errors.New("coproc: expected a command")
	}
	body, after := script.program[0], script.program[1:]
	if items, ok := body.(listNode); ok && len(items.value.items) > 0 {
		first := items.value.items[0]
		if first.background {
			return nil, true, errors.New("coproc: the command is already in the background")
		}
		body = listNode{value: listOf(first)}
		if len(items.value.items) > 1 {
			after = append([]programNode{listNode{value: list{items: items.value.items[1:]}}}, after...)
		}
	}
	return append([]programNode{coprocNode{name: name, body: body}}, after...), true, nil
}

func listOf(item listItem) list { return list{items: []listItem{item}} }

// lineNodes parses a line that is neither a compound nor a definition: a coprocess and
// what the line runs after it, or else a list.
func lineNodes(line, raw string, budget *parseBudget, depth int) ([]programNode, error) {
	if nodes, ok, err := coprocLine(line, budget, depth); ok {
		return nodes, err
	}
	parsed, err := parseTypedLineWithBudget(raw, budget, depth)
	if err != nil || len(parsed.items) == 0 {
		return nil, err
	}
	return []programNode{listNode{value: parsed}}, nil
}

// splitCompoundAfterPrefix is splitCompoundAfterOperator, and `coproc [NAME]` in front of
// a line as one more kind of prefix, for the parser that finds where a loop, an if or a
// case begins and ends: `coproc L while read l; do ...; done` is a loop that is a
// coprocess. The name comes back as the prefix, and "coproc" as the operator.
func splitCompoundAfterPrefix(line string) (string, string, string, bool) {
	if name, rest, ok := functionHeaderBeforeCompound(line); ok {
		return name, "()", rest, true
	}
	if rest, ok := strings.CutPrefix(line, "coproc "); ok {
		name, rest := coprocName(strings.TrimSpace(rest))
		if beginsWithCompoundKeyword(rest) {
			return name, "coproc", rest, true
		}
	}
	return splitCompoundAfterOperator(line)
}

// coprocName is the name, and the command after it. A name is taken only before a compound
// command, as in bash: in `coproc cat file`, cat is the command.
func coprocName(rest string) (string, string) {
	word, after, found := strings.Cut(rest, " ")
	if !found || !isVariableName(word) {
		return "COPROC", rest
	}
	after = strings.TrimSpace(after)
	for _, opener := range []string{"{", "(", "while ", "until ", "for ", "if ", "case "} {
		if strings.HasPrefix(after, opener) {
			return word, after
		}
	}
	return "COPROC", rest
}

func (r Runtime) executeCoproc(ctx context.Context, node coprocNode, savedStatus int) lineResult {
	fromJob, toShell, err := os.Pipe()
	if err != nil {
		return r.coprocFailure(err)
	}
	fromShell, toJob, err := os.Pipe()
	if err != nil {
		return r.coprocFailure(errors.Join(err, fromJob.Close(), toShell.Close()))
	}
	// The job's ends first, on descriptors the group moves to 0 and 1 and then closes.
	stdin := r.fds.lowestFree(10)
	bound := r.fds.bindOwnedReader(stdin, fromShell)
	stdout := r.fds.lowestFree(10)
	bound = errors.Join(bound, r.fds.bindOwnedWriter(stdout, toShell))
	if bound != nil {
		return r.coprocFailure(errors.Join(bound, r.fds.close(stdin), r.fds.close(stdout), fromJob.Close(), toJob.Close()))
	}
	group := braceGroup{body: Script{program: []programNode{node.body}}, redirects: []redirectOperation{
		{kind: redirectDup, target: 0, source: stdin}, {kind: redirectDup, target: 1, source: stdout},
		{kind: redirectClose, target: stdin}, {kind: redirectClose, target: stdout},
	}}
	job := listNode{value: listOf(listItem{value: andOr{pipelines: []pipeline{{commands: []commandNode{group}}}}})}
	result := r.executeNode(ctx, backgroundNode{value: job}, savedStatus)
	// The job has its own copies now; the shell's go, so the job's stdout ends with it.
	closed := errors.Join(r.fds.close(stdin), r.fds.close(stdout))
	if result.status != 0 || closed != nil {
		return r.coprocFailure(errors.Join(closed, fromJob.Close(), toJob.Close()))
	}
	// Then the shell's ends, which the job never had.
	read := r.fds.freeOr(63)
	bound = r.fds.bindOwnedReader(read, fromJob)
	written := r.fds.freeOr(60)
	if err := errors.Join(bound, r.fds.bindOwnedWriter(written, toJob)); err != nil {
		return r.coprocFailure(err)
	}
	r.arrays.set(node.name, []string{strconv.Itoa(read), strconv.Itoa(written)})
	r.vars[node.name+"_PID"] = r.vars["!"]
	r.markVarMutation(node.name)
	r.markVarMutation(node.name + "_PID")
	if record, ok := r.jobScope.newest(); ok {
		// Set and read only by this shell's own goroutine; see disposeCoprocs.
		record.coproc = &coprocState{name: node.name, table: r.fds, read: read, written: written}
	}
	return lineResult{}
}

// freeOr is fd if nothing holds it, and otherwise the lowest free descriptor from 10.
func (t *fdTable) freeOr(fd int) int {
	if _, err := t.lookup(fd); err == nil {
		return t.lowestFree(10)
	}
	return fd
}

// newest is the job registered last.
func (s *jobScope) newest() (*jobRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[s.next]
	return record, ok
}

func (r Runtime) coprocFailure(err error) lineResult {
	fmt.Fprintf(r.streams.Stderr, "coproc: %v\n", err)
	return lineResult{status: 1}
}

// disposeCoprocs is a reaped coprocess's end: its descriptors closed and its names unset.
func (r Runtime) disposeCoprocs(records []*jobRecord) {
	for _, record := range records {
		if state := record.coproc; state != nil {
			_ = state.table.close(state.read)
			_ = state.table.close(state.written)
			r.unsetName(state.name)
			r.unsetName(state.name + "_PID")
		}
	}
}

// coproc prints a coprocess as the group it runs, which reads back as the same thing.
func (p *scriptPrinter) coproc(node coprocNode) {
	p.line("coproc " + node.name + " {")
	p.depth++
	p.statement(node.body)
	p.depth--
	p.line("}")
}
