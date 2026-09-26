package oilsspec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Subject is a shell the cases run with.
type Subject struct {
	// Label names the shell whose expectations it is held to, as Oils names shells.
	Label string
	// Program is the shell's executable. Name is what $SH holds: the name PATH finds the
	// shell by, which cases compare with bash, dash and the rest.
	Program, Name string
	// Path is PATH's directories, the helpers' first.
	Path []string
	// Env is anything more the shell is given.
	Env []string
	// Timeout is how long a case may run before it, and all it started, are ended: ten
	// seconds when it is zero.
	Timeout time.Duration
}

func (s Subject) timeout() time.Duration {
	if s.Timeout == 0 {
		return 10 * time.Second
	}
	return s.Timeout
}

// Run is what one run of a case printed and returned, and whether it ran out of time.
type Run struct {
	Output
	TimedOut bool
}

// RunCase runs code with the subject as sh_spec.py runs a case: on the shell's stdin, in
// a working directory of its own that TMP names too, with an environment made for it
// rather than inherited, and HOME the working directory, so nothing of the user's is
// read. repoRoot is what $REPO_ROOT names. Whatever the case leaves running is ended when
// it is over.
func (s Subject) RunCase(ctx context.Context, code, dir, repoRoot string) (Run, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout())
	defer cancel()
	cmd := exec.CommandContext(ctx, s.Program)
	// Its own name as it knows it is $SH's, as when sh_spec.py runs `bash`: a message it
	// prefixes with it, such as "bash: line 2: ...", is compared.
	cmd.Args = []string{s.Name}
	cmd.Dir = dir
	cmd.Env = append([]string{
		"PATH=" + strings.Join(s.Path, string(os.PathListSeparator)),
		"SH=" + s.Name,
		"TMP=" + filepath.ToSlash(dir),
		"HOME=" + filepath.ToSlash(dir),
		"REPO_ROOT=" + filepath.ToSlash(repoRoot),
		"LC_ALL=C.UTF-8",
	}, s.Env...)
	cmd.Stdin = strings.NewReader(code)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	tree, err := newProcessTree(cmd)
	if err != nil {
		return Run{}, err
	}
	defer tree.close()
	cmd.Cancel = tree.kill
	// A job the case leaves behind can hold its output open once the shell is gone. What
	// it writes in the next few seconds is kept, as sh_spec.py, waiting for the end of
	// the output, keeps it; then the case is over.
	cmd.WaitDelay = 3 * time.Second
	if err := cmd.Start(); err != nil {
		return Run{}, err
	}
	// A shell that ends at once, on a parse error, can be gone before it is held; the tree
	// then ends it alone if it has to.
	tree.attach(cmd.Process.Pid)
	waited := cmd.Wait()
	_ = tree.kill()
	run := Run{
		Output:   Output{Stdout: stdout.String(), Stderr: stderr.String(), Status: exitStatus(cmd.ProcessState)},
		TimedOut: ctx.Err() != nil,
	}
	var exit *exec.ExitError
	if waited != nil && !run.TimedOut && !errors.As(waited, &exit) && !errors.Is(waited, exec.ErrWaitDelay) {
		return run, waited
	}
	return run, nil
}

// exitStatus is the status as sh_spec.py reads it from Python's wait: the negated signal
// for a shell a signal ended, and its exit status otherwise.
func exitStatus(state *os.ProcessState) int {
	if status, ok := state.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return -int(status.Signal())
	}
	return state.ExitCode()
}

// CaseResult is how one case went for a subject: its run scored against what the case
// expects of bash and of ash, and what differed from the subject's own label's.
type CaseResult struct {
	ID        string   `json:"id"`
	Line      int      `json:"line"`
	Bash      Result   `json:"bash"`
	Ash       Result   `json:"ash"`
	Unmatched []string `json:"unmatched,omitempty"`
}

// RunSpec runs a file's cases in order, as sh_spec.py does, each in its own directory
// under dir, with the _tmp directory in it that legacy_tmp_dir files expect, and scores
// each against bash's expectations and ash's.
func (s Subject) RunSpec(ctx context.Context, spec Spec, dir, repoRoot string) ([]CaseResult, error) {
	results := make([]CaseResult, 0, len(spec.Cases))
	for i, c := range spec.Cases {
		caseDir := filepath.Join(dir, strconv.Itoa(i))
		if err := os.MkdirAll(caseDir, 0o755); err != nil {
			return nil, err
		}
		if spec.Metadata["legacy_tmp_dir"] != "" {
			if err := os.Mkdir(filepath.Join(caseDir, "_tmp"), 0o755); err != nil {
				return nil, err
			}
		}
		run, err := s.RunCase(ctx, c.Code, caseDir, repoRoot)
		if err != nil {
			return nil, fmt.Errorf("line %d, %s: %w", c.Line, c.ID, err)
		}
		result := CaseResult{ID: c.ID, Line: c.Line}
		for _, label := range []string{"bash", "ash"} {
			scored, unmatched, err := s.score(c, label, run)
			if err != nil {
				return nil, err
			}
			if label == "bash" {
				result.Bash = scored
			} else {
				result.Ash = scored
			}
			if label == s.Label {
				result.Unmatched = unmatched
			}
		}
		results = append(results, result)
		_ = os.RemoveAll(caseDir)
	}
	return results, nil
}

func (s Subject) score(c Case, label string, run Run) (Result, []string, error) {
	if run.TimedOut {
		return Timeout, []string{fmt.Sprintf("ran out of its %v", s.timeout())}, nil
	}
	expected, err := c.Expect(label)
	if err != nil {
		return Fail, nil, err
	}
	result, unmatched := expected.Check(run.Output)
	return result, unmatched, nil
}
