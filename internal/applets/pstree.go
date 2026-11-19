package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/rivo/uniseg"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// pstree, psmisc's (src/pstree.c), from the system's process table, parentage as proc.Tree has it
// on Windows, where a parent must be older than its child. busybox-w32 has no pstree.
//
// dump_tree's drawing: the first child on its parent's line after ─┬─ or ───, each other indented
// to the parent's name and pids and hung from ├─ or └─, │ for an ancestor with more to come;
// identical siblings folded into N*[...]; a process's other threads as {name} children; -p's pids,
// which stop the folding; -a's arguments, a process a line; -A's ASCII and -U's UTF-8, UTF-8 on a
// terminal and ASCII into a pipe as psmisc chooses; -n by pid rather than name; -T no threads; -c
// no folding; a line cut at COLUMNS, the terminal's width or 132 with a + but with -l; and a PID
// for that process's tree alone. Threads stay folded under -p: Windows' have no tid to show here,
// and a process runs dozens. A USER is refused with a pointer to ps -u.

type pstreeSource struct {
	processes func() ([]proc.Process, error)
	arguments func(proc.Process) string
}

type pstreeNode struct {
	name, arguments string
	pid             int
	thread          bool
	children        []*pstreeNode
}

type pstreeSymbols struct{ empty, branch, vertical, last, single, first string }

var (
	pstreeASCII = pstreeSymbols{"  ", "|-", "| ", "`-", "---", "-+-"}
	pstreeUTF8  = pstreeSymbols{"  ", "├─", "│ ", "└─", "───", "─┬─"}
)

func newPstreeApplet() Applet {
	return simpleApplet{name: "pstree", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		cache := proc.NewDetailCache()
		source := pstreeSource{
			processes: func() ([]proc.Process, error) {
				snapshot, err := proc.NewSampler().Sample(false)
				return snapshot.Processes, err
			},
			arguments: func(process proc.Process) string { return pstreeArguments(cache.Lookup(process).CommandLine) },
		}
		return runPstree(ctx, args, source, stdout)
	}}
}

func runPstree(ctx context.Context, args []string, source pstreeSource, stdout io.Writer) error {
	options, operands, err := parseAppletOptions(ctx, args, "pacAUlnT", "")
	if err != nil {
		return err
	}
	run := &pstreeRun{symbols: pstreeASCII, limit: 132, truncate: !options.has('l'), pids: options.has('p'),
		arguments: options.has('a'), compact: !options.has('c') && !options.has('p')}
	if stdoutIsTerminal(stdout) {
		run.symbols = pstreeUTF8
	}
	switch {
	case options.has('U'):
		run.symbols = pstreeUTF8
	case options.has('A'):
		run.symbols = pstreeASCII
	}
	if columns, ok := ProcessViewFromContext(ctx).LookupEnv("COLUMNS"); ok {
		if width, err := strconv.Atoi(columns); err == nil && width > 0 {
			run.limit = width
		}
	} else if width := psTerminalWidth(stdout); width > 0 {
		run.limit = width
	}
	processes, err := source.processes()
	if err != nil {
		return err
	}
	byName := func(a, b proc.Process) bool { return a.Name < b.Name }
	if options.has('n') {
		byName = func(a, b proc.Process) bool { return a.PID < b.PID }
	}
	roots := pstreeNodes(proc.Tree(processes, byName), !options.has('T'), options.has('a'), source.arguments)
	if len(operands) > 0 {
		pid, err := strconv.Atoi(operands[0])
		if err != nil {
			return fmt.Errorf("%s: pstree of a USER is not here; ps -u USER lists what a user runs", operands[0])
		}
		if roots = pstreeFind(roots, pid); roots == nil {
			return ExitStatusMessage(1, errors.New("No processes found."))
		}
	}
	run.out = bufio.NewWriter(stdout)
	for _, root := range roots {
		run.dump(root, 0, 1, false, true, 0)
	}
	return run.out.Flush()
}

// pstreeNodes are proc.Tree's nodes as pstree draws them, a process's other threads as {name}
// children after its own, in the order proc.Tree gave.
func pstreeNodes(nodes []*proc.Node, threads, arguments bool, argumentsOf func(proc.Process) string) []*pstreeNode {
	var out []*pstreeNode
	for _, node := range nodes {
		found := &pstreeNode{name: node.Process.Name, pid: node.Process.PID}
		if arguments {
			found.arguments = argumentsOf(node.Process)
		}
		found.children = pstreeNodes(node.Children, threads, arguments, argumentsOf)
		for range max(0, node.Process.Threads-1) {
			if threads {
				found.children = append(found.children, &pstreeNode{name: "{" + node.Process.Name + "}", thread: true})
			}
		}
		out = append(out, found)
	}
	return out
}

func pstreeFind(nodes []*pstreeNode, pid int) []*pstreeNode {
	for _, node := range nodes {
		if node.pid == pid && !node.thread {
			return []*pstreeNode{node}
		}
		if found := pstreeFind(node.children, pid); found != nil {
			return found
		}
	}
	return nil
}

// pstreeEqual is tree_equal: the same name and the same children, in order.
func pstreeEqual(a, b *pstreeNode) bool {
	return a.name == b.name && a.arguments == b.arguments && slices.EqualFunc(a.children, b.children, pstreeEqual)
}

// pstreeArguments is a command line without its program, which pstree -a writes after the name.
func pstreeArguments(command string) string {
	command = strings.TrimSpace(command)
	if strings.HasPrefix(command, `"`) {
		if end := strings.Index(command[1:], `"`); end >= 0 {
			return strings.TrimSpace(command[end+2:])
		}
		return ""
	}
	_, rest, _ := strings.Cut(command, " ")
	return strings.TrimSpace(rest)
}

func pstreeWidth(text string) int { return uniseg.StringWidth(text) }
