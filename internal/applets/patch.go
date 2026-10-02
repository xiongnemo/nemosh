package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// patch: applies what diff writes, as busybox's editors/patch.c does, but for where a hunk may
// land; see patch_apply.go.
//
// The pair is kept together deliberately. Shipping patch against a diff whose
// output shape later changed would break it silently, so the tests round-trip the
// two against each other rather than testing each alone.
//
// What busybox's does, this does: `patching file F` on stdout, `creating F` for a file whose old
// side is /dev/null, parent directories and all, and for one whose new side is /dev/null an
// emptied file, or with -E `removing F`. The file patched is the one the +++ line names, the ---
// line's with -R, and without -p only its last component is kept, as POSIX has it. ORIGFILE names
// the file for every hunk, and PATCHFILE, or -i's, is read in place of stdin. A file whose hunk
// does not apply is left as it was and the next is patched still, and the status is 1; one that
// cannot be opened ends it. Only garbage in the patch is refused, as GNU refuses it and busybox
// does not; nothing at all is no patch to apply.

// patchLongOptions are busybox's long forms. --dry-run has no letter of its own there and has
// none here, and --get takes an argument its -g does not.
var patchLongOptions = map[string]string{
	"reverse": "R", "unified": "u", "strip": "p", "input": "i", "forward": "N",
	"remove-empty-files": "E", "force": "f", "get": "\x02", "dry-run": "\x01",
	"backup-if-mismatch": "\x03", "no-backup-if-mismatch": "\x03",
}

func newPatchApplet() Applet {
	return simpleApplet{name: "patch", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletLongOptions(ctx, args, patchLongOptions, "RuNEfg\x01\x03", "pi\x02")
		if err != nil {
			return err
		}
		run := patchRun{ctx: ctx, view: ProcessViewFromContext(ctx), stdout: stdout, strip: -1,
			reverse: options.has('R'), removeEmpty: options.has('E'), dryRun: options.has('\x01')}
		if options.has('p') {
			if run.strip, err = strconv.Atoi(options.value('p')); err != nil {
				return fmt.Errorf("invalid number '%s'", options.value('p'))
			}
		}
		input := options.value('i')
		if len(operands) > 0 {
			run.target = operands[0]
		}
		if !options.has('i') && len(operands) > 1 {
			input = operands[1]
		}
		source := stdin
		if input != "" {
			file, err := OpenProcessOperand(ctx, run.view, input, stdin)
			if err != nil {
				return cannotOpen(input, err)
			}
			defer file.Close()
			source = file
		}
		content, err := io.ReadAll(source)
		if err != nil {
			return err
		}
		sets, err := parseUnifiedPatch(strings.NewReader(string(content)))
		if err != nil {
			return err
		}
		if len(sets) == 0 && strings.TrimSpace(string(content)) != "" {
			return fmt.Errorf("only garbage was found in the patch input")
		}
		return run.apply(sets)
	}}
}

// patchRun is one patch's options, applied to each file its diff names.
type patchRun struct {
	ctx                          context.Context
	view                         ProcessView
	stdout                       io.Writer
	reverse, removeEmpty, dryRun bool
	// strip is -p, or -1 without it, which keeps only a name's last component.
	strip int
	// target is ORIGFILE, which stands for every file the diff names.
	target string
}

// patchHunkError is a file's hunk that does not apply, which the next file is patched past.
type patchHunkError struct{ error }

func (r patchRun) apply(sets []patchSet) error {
	failed := false
	for _, set := range sets {
		if len(set.hunks) == 0 {
			continue
		}
		err := r.patchFile(set)
		if err == nil {
			continue
		}
		if !errors.As(err, new(patchHunkError)) || !reportOperand(r.ctx, err) {
			return err
		}
		failed = true
	}
	if failed {
		return ExitStatus(1)
	}
	return nil
}

// patchFile applies one file's hunks. Its first hunk says whether it creates the file, from an
// old side of nothing, or empties it, to a new side of nothing, as busybox reads it.
func (r patchRun) patchFile(set patchSet) error {
	oldName, newName := set.oldName, set.newName
	first := set.hunks[0]
	oldEnd, newEnd := first.oldStart+first.oldLen, first.newStart+first.newLen
	if r.reverse {
		oldName, newName, oldEnd, newEnd = newName, oldName, newEnd, oldEnd
	}
	emptying := newName == patchDevNull || newEnd == 0
	creating := !emptying && (oldName == patchDevNull || oldEnd == 0)
	name := newName
	if emptying {
		name = oldName
	}
	target, native, err := r.resolve(name)
	if err != nil {
		return err
	}
	original := patchText{newline: true}
	switch {
	case creating:
		fmt.Fprintf(r.stdout, "creating %s\n", target)
		if _, err := os.Lstat(native); err == nil && !r.dryRun {
			return cannotOpen(target, fs.ErrExist)
		}
	default:
		verb := "patching file"
		if emptying && r.removeEmpty {
			verb = "removing"
		}
		fmt.Fprintf(r.stdout, "%s %s\n", verb, target)
		content, err := os.ReadFile(native)
		if err != nil {
			return cannotOpen(target, err)
		}
		original = readPatchText(string(content))
	}
	patched, err := applyHunks(original, set.hunks, r.reverse)
	if err != nil {
		return patchHunkError{fmt.Errorf("%s: %v", target, err)}
	}
	return r.write(target, native, patched, creating, emptying)
}

// resolve is the file a name from the diff stands for: ORIGFILE when there is one, and otherwise
// the name stripped by -p and checked as an archive entry is, since `--- ../../etc/passwd` is the
// same attack.
func (r patchRun) resolve(name string) (string, string, error) {
	target := r.target
	if target == "" {
		safe, err := safeArchivePath(stripPatchName(name, r.strip))
		if err != nil {
			return "", "", err
		}
		target = safe
	}
	native, err := resolveHostPath(r.view, target)
	if err != nil {
		return "", "", operandFailure(target, err)
	}
	return target, native, nil
}

func (r patchRun) write(target, native string, patched patchText, creating, emptying bool) error {
	if r.dryRun {
		return nil
	}
	if emptying && r.removeEmpty && len(patched.lines) == 0 {
		if err := os.Remove(native); err != nil {
			return operandFailure(target, err)
		}
		return nil
	}
	if creating {
		if err := os.MkdirAll(filepath.Dir(native), 0o777); err != nil {
			return operandFailure(target, err)
		}
	}
	if err := os.WriteFile(native, []byte(patched.String()), 0o666); err != nil {
		return operandFailure(target, err)
	}
	return nil
}

// stripPatchName is -p: the first strip components of name go, a run of slashes counting as
// one, as busybox's loop takes them. Without -p strip is -1 and every directory goes, as POSIX
// has it and busybox does, and a name with fewer components than -p asks for loses all it has.
func stripPatchName(name string, strip int) string {
	name = strings.ReplaceAll(name, `\`, "/")
	for count := 0; strip < 0 || count < strip; count++ {
		at := strings.IndexByte(name, '/')
		if at < 0 {
			break
		}
		name = strings.TrimLeft(name[at+1:], "/")
	}
	return name
}
