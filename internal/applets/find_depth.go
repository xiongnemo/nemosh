package applets

import (
	"io/fs"
	"os"
	"path/filepath"
)

// walkFindDepthFirst is -depth's walk: a directory's entries before the directory itself, as
// busybox's ACTION_DEPTHFIRST has it, so `find . -depth` ends with `.`. The entries come in
// name order, as the walk without -depth has them. -prune stops nothing here, as in both
// references: a directory's entries have been walked by the time it is seen.
func walkFindDepthFirst(run *findRun, displayRoot, hostRoot string, expression findExpression) error {
	info, err := os.Lstat(hostRoot)
	if err != nil {
		return operandFailure(displayRoot, err)
	}
	return walkFindBelow(run, displayRoot, hostRoot, hostRoot, fs.FileInfoToDirEntry(info), expression)
}

func walkFindBelow(run *findRun, displayRoot, hostRoot, path string, entry fs.DirEntry, expression findExpression) error {
	display, depth, err := findDisplayPath(displayRoot, hostRoot, path)
	if err != nil {
		return err
	}
	candidate := findCandidate{display: display, host: path, entry: entry, depth: depth}
	if entry.IsDir() && !expression.prunes(candidate) && !run.leavesVolume(expression, path) {
		entries, err := os.ReadDir(path)
		if err != nil {
			return operandFailure(display, err)
		}
		for _, child := range entries {
			if err := walkFindBelow(run, displayRoot, hostRoot, filepath.Join(path, child.Name()), child, expression); err != nil {
				return err
			}
		}
	}
	return expression.evaluate(candidate, run)
}
