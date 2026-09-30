package applets

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// cpFlags are the FILEUTILS flags busybox's cp hands copy_file (include/libbb.h:535-562).
type cpFlags struct {
	preserve, recurse, dereference, derefTop bool
	interactive, noClobber                   bool
	hardLink, softLink, noTargetDir          bool
	update, verbose, removeDest, parents     bool
	targetDir                                string
}

// cpRun is one cp: its flags, where it asks and reports, and what it has made so far. applet
// names it in messages, cp unless mv is copying across volumes.
type cpRun struct {
	applet         string
	flags          cpFlags
	view           ProcessView
	stdin          io.Reader
	stdout, stderr io.Writer
	umask          uint32
	// made are the directories this copy created or copied into, which is how a directory copied
	// into itself is noticed rather than copied again inside its copy, for ever.
	made   []os.FileInfo
	failed bool
}

// copy is libbb's copy_file (libbb/copy_file.c): it copies source to dest and answers whether
// the copy was made, or declined as -n, -u or an answer to -i asked. Every failure is named on
// stderr and makes cp's status 1. top is whether source was named on the command line, which is
// where -H follows a link.
func (r *cpRun) copy(source, dest pathOperand, top bool) bool {
	deref := r.flags.dereference || top && r.flags.derefTop
	stat := os.Lstat
	if deref {
		stat = os.Stat
	}
	sourceInfo, err := r.statSource(source, stat)
	if err != nil {
		if r.flags.hardLink || r.flags.softLink {
			return r.makeLink(source, dest)
		}
		return r.fail(cannotStat(source.operand, err))
	}
	if dest.device {
		return r.copyToDevice(source, dest, sourceInfo)
	}
	destInfo, err := os.Lstat(dest.host)
	destExists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return r.fail(cannotStat(dest.operand, err))
	}
	if destExists {
		if os.SameFile(sourceInfo, destInfo) {
			return r.fail(fmt.Errorf("'%s' and '%s' are the same file", source.operand, dest.operand))
		}
		if r.flags.noClobber {
			return true
		}
	}
	if sourceInfo.IsDir() {
		return r.copyDirectory(source, dest, sourceInfo, destInfo)
	}
	if destExists && r.flags.update && !sourceInfo.ModTime().After(destInfo.ModTime()) {
		return true
	}
	if destExists && r.flags.removeDest {
		if answer := r.askAndUnlink(dest, nil); answer != unlinked {
			return answer == unlinkDeclined
		}
		destExists = false
	}
	if r.flags.hardLink || r.flags.softLink {
		return r.makeLink(source, dest)
	}
	if !deref && sourceInfo.Mode()&os.ModeSymlink != 0 || r.flags.recurse && !sourceInfo.Mode().IsRegular() {
		return r.copyLink(source, dest, sourceInfo, destExists)
	}
	return r.copyRegular(source, dest, sourceInfo)
}

// copyDirectory makes dest and copies what source holds into it, going on past what it cannot
// copy. Without -R it is `omitting directory`.
func (r *cpRun) copyDirectory(source, dest pathOperand, sourceInfo, destInfo os.FileInfo) bool {
	if !r.flags.recurse {
		return r.fail(omittingDirectory(source.operand))
	}
	for _, made := range r.made {
		if os.SameFile(made, sourceInfo) {
			return r.fail(fmt.Errorf("recursion detected, omitting directory '%s'", source.operand))
		}
	}
	created := destInfo == nil
	if !created && !destInfo.IsDir() {
		return r.fail(fmt.Errorf("target '%s' is not a directory", dest.operand))
	}
	if created {
		if err := os.Mkdir(dest.host, sourceInfo.Mode().Perm()|0o700); err != nil {
			return r.fail(cannotCreateDirectory(dest.operand, err))
		}
		var err error
		if destInfo, err = os.Lstat(dest.host); err != nil {
			return r.fail(cannotStat(dest.operand, err))
		}
	}
	r.made = append(r.made, destInfo)
	entries, err := os.ReadDir(source.host)
	ok := err == nil
	if err != nil {
		r.fail(cannotOpen(source.operand, err))
	}
	for _, entry := range entries {
		name := entry.Name()
		inner := pathOperand{host: filepath.Join(source.host, name), operand: joinOperand(source.operand, name)}
		into := pathOperand{host: filepath.Join(dest.host, name), operand: joinOperand(dest.operand, name)}
		ok = r.copy(inner, into, false) && ok
	}
	if created && !r.flags.preserve {
		_ = applyPermissions(dest.host, bitsOfFileMode(sourceInfo.Mode())&^r.umask, true)
	}
	r.finish(source, dest, sourceInfo)
	return ok
}

// copyRegular copies a file's bytes. A destination that is there is removed and made afresh,
// as busybox-w32 does whether or not -f was given (FEATURE_NON_POSIX_CP): it opens with O_EXCL
// and unlinks what is in the way, read-only or not, so the copy is a new file and never the
// target of a link that stood at dest.
func (r *cpRun) copyRegular(source, dest pathOperand, info os.FileInfo) bool {
	reader, err := r.openSource(source)
	if err != nil {
		return r.fail(cannotOpen(source.operand, err))
	}
	defer reader.Close()
	mode := info.Mode().Perm()
	if !info.Mode().IsRegular() {
		mode = 0o666
	}
	mode = maskedMode(mode, r.umask)
	writer, err := os.OpenFile(dest.host, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		if answer := r.askAndUnlink(dest, err); answer != unlinked {
			return answer == unlinkDeclined
		}
		if writer, err = os.OpenFile(dest.host, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode); err != nil {
			return r.fail(cannotOpen(dest.operand, err))
		}
	}
	_, copyErr := io.Copy(writer, reader)
	if closeErr := writer.Close(); copyErr == nil && closeErr != nil {
		copyErr = fmt.Errorf("error writing to '%s': %s", dest.operand, causeText(closeErr))
	}
	if copyErr != nil {
		return r.fail(copyErr)
	}
	if info.Mode().IsRegular() {
		r.finish(source, dest, info)
	}
	return true
}

// copyLink copies a symbolic link as one, which -d, -P and -a ask for.
func (r *cpRun) copyLink(source, dest pathOperand, info os.FileInfo, destExists bool) bool {
	if destExists {
		if answer := r.askAndUnlink(dest, fs.ErrExist); answer != unlinked {
			return answer == unlinkDeclined
		}
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return r.fail(fmt.Errorf("unrecognized file '%s' with mode %x", source.operand, uint32(info.Mode())))
	}
	target, err := os.Readlink(source.host)
	if err != nil {
		return r.fail(fmt.Errorf("%s: cannot read link: %s", source.operand, causeText(err)))
	}
	if err := os.Symlink(target, dest.host); err != nil {
		return r.fail(fmt.Errorf("cannot create symlink '%s' to '%s': %s", dest.operand, target, causeText(err)))
	}
	if r.flags.preserve {
		r.preserveOwner(dest, info)
	}
	r.report(source, dest)
	return true
}

// makeLink is -l and -s: a hard or a symbolic link at dest rather than a copy. A symbolic one
// holds SOURCE as written, to be read from where the link is, as ln -s makes it.
func (r *cpRun) makeLink(source, dest pathOperand) bool {
	link := func() error { return os.Link(source.host, dest.host) }
	if r.flags.softLink {
		link = func() error { return os.Symlink(filepath.FromSlash(source.operand), dest.host) }
	}
	if err := link(); err != nil {
		if answer := r.askAndUnlink(dest, err); answer != unlinked {
			return answer == unlinkDeclined
		}
		if err := link(); err != nil {
			return r.fail(fmt.Errorf("cannot create link '%s': %s", dest.operand, causeText(err)))
		}
	}
	return true
}

// omittingDirectory is the refusal when -r was not given. busybox words it with no errno
// attached, because there is no failed call to report: the operand was simply not the kind
// this can take.
func omittingDirectory(operand string) error {
	return fmt.Errorf("omitting directory '%s'", operand)
}
