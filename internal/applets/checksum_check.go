package applets

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"strings"
)

// checkSums is busybox's -c (coreutils/md5_sha1_sum.c). Each FILE is a list of lines, each a
// hash, a blank and a name -- after one more blank or a `*` when there is one, the text and the
// binary spellings, and coreutils 9.1 writes neither. A name is OK when its hash is the line's in
// either case, and FAILED when it is not, when it cannot be opened, which is said on stderr too,
// or when the line has no blank at all, which -w says as `invalid format`. A list with a failure
// ends with `WARNING: N of M computed checksums did NOT match`, and one with no lines at all is
// `FILE: no checksum lines found`. -s prints no OK, FAILED or WARNING, and after any failure the
// status is 1.
//
// It was GNU's, nearly: a blank or malformed line was skipped, a name that could not be opened was
// `FAILED open or read`, one `N computed checksum did NOT match` counted every list at once, a
// hash in capitals never matched, an empty list passed, and -s was refused. Each answer here is
// busybox-w32's, measured.
func checkSums(ctx context.Context, applet string, newHash func() hash.Hash, paths []string, stdin io.Reader, stdout, stderr io.Writer, silent, warn bool) error {
	if len(paths) == 0 {
		paths = []string{"-"}
	}
	check := sumChecker{ctx: ctx, view: ProcessViewFromContext(ctx), applet: applet, newHash: newHash,
		stdin: stdin, stdout: stdout, stderr: stderr, silent: silent, warn: warn}
	failed := false
	for _, path := range paths {
		list, err := OpenProcessOperand(ctx, check.view, path, stdin)
		if err != nil {
			return operandFailure(path, err)
		}
		total, failures, err := check.list(list)
		list.Close()
		if err != nil {
			return err
		}
		switch {
		case total == 0:
			fmt.Fprintf(stderr, "%s: %s: no checksum lines found\n", applet, path)
		case failures > 0 && !silent:
			fmt.Fprintf(stderr, "%s: WARNING: %d of %d computed checksums did NOT match\n", applet, failures, total)
		}
		failed = failed || total == 0 || failures > 0
	}
	if failed {
		return ExitStatus(1)
	}
	return nil
}

// sumChecker is what checking a list's lines needs.
type sumChecker struct {
	ctx            context.Context
	view           ProcessView
	applet         string
	newHash        func() hash.Hash
	stdin          io.Reader
	stdout, stderr io.Writer
	silent, warn   bool
}

// list checks each of a list's lines, answering how many there were and how many failed.
func (c sumChecker) list(list io.Reader) (total, failures int, err error) {
	lines := bufio.NewScanner(list)
	lines.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	lines.Split(scanLineWithEnding)
	for lines.Scan() {
		line, _ := splitLineEnding(lines.Text())
		total++
		if !c.line(line) {
			failures++
		}
	}
	return total, failures, lines.Err()
}

// line checks one line, answering whether its name was OK.
func (c sumChecker) line(line string) bool {
	want, name, found := strings.Cut(line, " ")
	if !found {
		if c.warn {
			fmt.Fprintf(c.stderr, "%s: invalid format\n", c.applet)
		}
		return false
	}
	if strings.HasPrefix(name, " ") || strings.HasPrefix(name, "*") {
		name = name[1:]
	}
	got, err := c.hash(name)
	if err != nil {
		fmt.Fprintf(c.stderr, "%s: %v\n", c.applet, err)
	}
	ok := err == nil && strings.EqualFold(got, want)
	if !c.silent {
		verdict := "FAILED"
		if ok {
			verdict = "OK"
		}
		fmt.Fprintf(c.stdout, "%s: %s\n", name, verdict)
	}
	return ok
}

// hash is name's hash in hex, or why there is none, as busybox's hash_file says it.
func (c sumChecker) hash(name string) (string, error) {
	file, err := OpenProcessOperand(c.ctx, c.view, name, c.stdin)
	if err != nil {
		return "", cannotOpen(name, err)
	}
	defer file.Close()
	digest := c.newHash()
	if _, err := io.Copy(digest, file); err != nil {
		return "", fmt.Errorf("cannot read '%s': %s", name, causeText(err))
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
