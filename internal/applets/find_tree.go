package applets

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// The walk find takes where filepath.WalkDir will not do: under -depth, which wants a
// directory's entries before the directory, and under -L and -H, which follow symbolic links.
//
// -depth is busybox's ACTION_DEPTHFIRST, so `find . -depth` ends with `.`. -prune stops nothing
// under it, as in both references: a directory's entries have been walked by the time it is
// seen. -L follows every link and -H the PATHs alone, as busybox stats them: a link to a
// directory is walked as the directory, -type sees what a link points at, and a dangling link is
// the link. The entries come in name order, as the walk takes them without either.

// walkFindTree walks one PATH so.
func walkFindTree(run *findRun, displayRoot, hostRoot string, expression findExpression) error {
	info, err := os.Lstat(hostRoot)
	if err != nil {
		return operandFailure(displayRoot, err)
	}
	return walkFindNode(run, displayRoot, hostRoot, hostRoot, fs.FileInfoToDirEntry(info), expression, nil)
}

// walkFindNode is one entry and, when it is a directory to go into, what it holds. ancestors are
// the directories above it, by identity, while links are followed, so a link back to one of them
// is not gone into again: busybox goes round until the path is too long, and GNU stops there.
func walkFindNode(run *findRun, displayRoot, hostRoot, path string, entry fs.DirEntry, expression findExpression, ancestors []fileID) error {
	display, depth, err := findDisplayPath(displayRoot, hostRoot, path)
	if err != nil {
		return err
	}
	if entry.Type()&fs.ModeSymlink != 0 && expression.follows(depth) {
		if info, err := os.Stat(path); err == nil {
			entry = fs.FileInfoToDirEntry(info)
		}
	}
	candidate := findCandidate{display: display, host: path, entry: entry, depth: depth}
	descend := entry.IsDir() && !expression.prunes(candidate) && !run.leavesVolume(expression, path)
	if !expression.depthFirst {
		run.pruned = false
		if err := expression.evaluate(candidate, run); err != nil {
			return err
		}
		descend = descend && !run.pruned
	}
	if descend {
		if err := walkFindEntries(run, displayRoot, hostRoot, path, display, expression, ancestors); err != nil {
			return err
		}
	}
	if expression.depthFirst {
		return expression.evaluate(candidate, run)
	}
	return nil
}

// walkFindEntries walks what the directory path holds, unless following links has come back
// round to a directory above it.
func walkFindEntries(run *findRun, displayRoot, hostRoot, path, display string, expression findExpression, ancestors []fileID) error {
	if expression.follow != 0 {
		if info, err := os.Stat(path); err == nil {
			if id, _, ok := fileIdentity(path, info); ok {
				if slices.Contains(ancestors, id) {
					return nil
				}
				ancestors = append(ancestors[:len(ancestors):len(ancestors)], id)
			}
		}
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return operandFailure(display, err)
	}
	for _, child := range entries {
		if err := walkFindNode(run, displayRoot, hostRoot, filepath.Join(path, child.Name()), child, expression, ancestors); err != nil {
			return err
		}
	}
	return nil
}
