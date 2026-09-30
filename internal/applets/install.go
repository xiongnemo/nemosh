package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
)

// install is busybox's (coreutils/install.c): `install [-cdDsp] [-o USER] [-g GRP] [-m MODE] [-t
// DIR] [SOURCE]... DEST` copies each SOURCE to DEST, or into it when DEST is a directory, and
// then gives the copy MODE, 0755 unless -m says otherwise, whatever the umask. The options:
//
//   - -d makes each operand a directory, with its parents, and gives it MODE;
//   - -D makes the directories DEST needs first, DEST itself with -t;
//   - -t DIR names the directory the SOURCEs go into;
//   - -p keeps each SOURCE's times, and -v names each copy and directory made;
//   - -o and -g set the owner and the group, a name or a number, where the platform has them;
//   - -s strips the copy with strip, which is a program rather than an applet, so it is not
//     found, as busybox says when it has none;
//   - -c and -b are taken and do nothing, as in busybox.
//
// Each copy is cp's, libbb's copy_file: it follows a SOURCE that is a link, replaces a DEST
// that is there, and refuses a directory. -d goes with neither -t nor -s. There was no install,
// so `install -m 755 tool bin/` and `install -d dir` in a script or a Makefile failed.
func newInstallApplet() Applet {
	return simpleApplet{name: "install", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletOptions(ctx, longOptionWords(args, installLongOptions), "cvbDdps", "gmot")
		if err != nil {
			return err
		}
		if options.has('d') && (options.has('t') || options.has('s')) {
			return errors.New("-d cannot be given with -t or -s")
		}
		view := ProcessViewFromContext(ctx)
		mode, err := installMode(options, processFileModeMask(view))
		if err != nil {
			return err
		}
		owner, err := installOwnership(options)
		if err != nil {
			return err
		}
		run := &cpRun{applet: "install", flags: cpFlags{dereference: true, preserve: options.has('p'), verbose: options.has('v')},
			stdin: stdin, stdout: stdout, stderr: stderr, umask: processFileModeMask(view)}
		install := installRun{cpRun: run, view: view, options: options, mode: mode, owner: owner}
		if options.has('d') {
			if len(operands) == 0 {
				return missingOperand()
			}
			for _, operand := range operands {
				install.directory(operand)
			}
			return run.status(nil)
		}
		last, sources, into := options.value('t'), operands, options.has('t')
		if !into {
			if len(operands) < 2 {
				return missingOperand()
			}
			last, sources = operands[len(operands)-1], operands[:len(operands)-1]
			if host, err := resolveHostPath(view, last); err == nil {
				info, statErr := os.Stat(host)
				into = statErr == nil && info.IsDir()
			}
		} else if len(sources) == 0 {
			return missingOperand()
		}
		for _, source := range sources {
			if err := ctx.Err(); err != nil {
				return err
			}
			install.file(source, last, into)
		}
		return run.status(nil)
	}}
}

// installLongOptions are the long forms busybox's install takes.
var installLongOptions = map[string]string{
	"verbose": "v", "directory": "d", "preserve-timestamps": "p", "strip": "s",
	"group": "g", "mode": "m", "owner": "o", "target-directory": "t",
}

// installRun is one install: cp's run, which names failures and keeps the status, and what
// each copy or directory is given once it is made.
type installRun struct {
	*cpRun
	view    ProcessView
	options appletOptions
	mode    os.FileMode
	owner   installOwner
}

// directory is -d's: operand made with its parents, then given the owner and MODE.
func (r installRun) directory(operand string) {
	if err := r.makeDirectories(operand); err != nil {
		r.fail(err)
		return
	}
	r.finishInstall(operand, true)
}

// file copies source to dest, or into it, and gives the copy the owner and MODE.
func (r installRun) file(source, dest string, into bool) {
	if r.options.has('D') {
		// -D DIR1/DIR2/F3 makes DIR1/DIR2, and -D -t DIR makes DIR. A failure is said and the
		// copy tried anyway, as busybox does: the copy then says what went wrong with it.
		leading := dest
		if !r.options.has('t') {
			leading = filepath.ToSlash(filepath.Dir(filepath.FromSlash(dest)))
		}
		if err := r.makeDirectories(leading); err != nil {
			fmt.Fprintf(r.stderr, "install: %v\n", err)
		}
	}
	if into {
		dest = joinOperand(dest, lastPathComponent(source))
	}
	sourceHost, err := resolveHostPath(r.view, source)
	if err != nil {
		r.fail(err)
		return
	}
	destHost, err := resolveHostPath(r.view, dest)
	if err != nil {
		r.fail(err)
		return
	}
	if !r.copy(pathOperand{host: sourceHost, operand: source}, pathOperand{host: destHost, operand: dest}, true) {
		return
	}
	if r.options.has('s') {
		r.fail(fmt.Errorf("strip: %s", causeText(os.ErrNotExist)))
	}
	r.finishInstall(dest, false)
}

// makeDirectories is mkdir -p's, and -v names each directory made.
func (r installRun) makeDirectories(path string) error {
	parents := appletOptions{given: map[byte]bool{'p': true, 'v': r.options.has('v')}}
	return makeDirectory(r.view, path, parents, 0o777, r.stdout)
}

// finishInstall gives what was made the owner -o and -g name and then MODE, which busybox sets
// whether -m was given or not.
func (r installRun) finishInstall(operand string, isDir bool) {
	host, err := resolveHostPath(r.view, operand)
	if err != nil {
		r.fail(err)
		return
	}
	if r.owner.given {
		if err := changeOwner(host, r.owner); err != nil {
			r.fail(fmt.Errorf("cannot change ownership of %s: %s", operand, causeText(err)))
		}
	}
	if err := applyPermissions(host, bitsOfFileMode(r.mode), isDir); err != nil {
		r.fail(fmt.Errorf("cannot change permissions of %s: %s", operand, causeText(err)))
	}
}

// installMode is -m's MODE, octal or symbolic, read against 0755, which is also what it is with
// no -m, as GNU's install 6.10 has it and busybox's copies.
func installMode(options appletOptions, umask uint32) (os.FileMode, error) {
	if !options.has('m') {
		return 0o755, nil
	}
	parsed, ok := applyChmodMode(options.value('m'), 0o755, umask, false)
	if !ok {
		return 0, fmt.Errorf("invalid mode '%s'", options.value('m'))
	}
	return fileModeOfBits(parsed), nil
}

// installOwner is who -o and -g give a copy to: the user and the group, each the invoking
// one's where only the other was named, as busybox fills them in.
type installOwner struct {
	given    bool
	uid, gid int
}

// installOwnership reads -o and -g: a number, or a name the platform knows. On Windows that is
// this session's account, or root, as busybox-w32's getpwnam knows them; a group has the same
// names, the one `id -gn` answers.
func installOwnership(options appletOptions) (installOwner, error) {
	owner := installOwner{given: options.has('o') || options.has('g'), uid: os.Getuid(), gid: os.Getgid()}
	var err error
	if options.has('o') {
		if owner.uid, err = accountID(options.value('o'), false); err != nil {
			return owner, err
		}
	}
	if options.has('g') {
		if owner.gid, err = accountID(options.value('g'), true); err != nil {
			return owner, err
		}
	}
	return owner, nil
}

// accountID is a user's or a group's number, from the number itself or the name.
func accountID(name string, group bool) (int, error) {
	if id, err := strconv.ParseUint(name, 10, 31); err == nil {
		return int(id), nil
	}
	kind := "user"
	if group {
		kind = "group"
	}
	if runtime.GOOS == "windows" {
		switch name {
		case rootUserName:
			return 0, nil
		case CurrentUserName(), accountName():
			return currentUserID(), nil
		}
		return 0, fmt.Errorf("unknown %s %s", kind, name)
	}
	var id string
	if group {
		found, err := user.LookupGroup(name)
		if err != nil {
			return 0, fmt.Errorf("unknown %s %s", kind, name)
		}
		id = found.Gid
	} else {
		found, err := user.Lookup(name)
		if err != nil {
			return 0, fmt.Errorf("unknown %s %s", kind, name)
		}
		id = found.Uid
	}
	return strconv.Atoi(id)
}

// changeOwner gives host to owner's user and group. Windows has no Unix owner to give, and
// busybox-w32's chown does nothing and succeeds.
func changeOwner(host string, owner installOwner) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return os.Lchown(host, owner.uid, owner.gid)
}
