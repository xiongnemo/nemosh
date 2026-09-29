package applets

import (
	"context"
	"fmt"
	"io"
	"os"
)

// listOperands lists the operands as busybox's ls does (coreutils/ls.c:1437-1451). Every operand
// is looked at first, and one that is not there is named on stderr. Then the ones that are not
// directories are sorted and laid out together, and then each directory, sorted the same way and
// headed by its name when there was more than one operand or -R, with a blank line between one
// block and the next. It answers whether every operand could be listed.
//
// Each operand was listed where it stood, a directory's entries with no name above them, so
// `ls src test` ran the two directories together and `ls dir notes.txt` set dir's entries beside
// notes.txt as though they had been named too.
func listOperands(ctx context.Context, stdout io.Writer, view ProcessView, targets []string, options lsOptions) (bool, error) {
	var files, directories []lsEntry
	listed := true
	for _, target := range targets {
		entry, err := statLsOperand(view, target)
		if err != nil {
			if len(targets) == 1 || !isOperandFailure(err) || !reportOperand(ctx, err) {
				return false, err
			}
			listed = false
			continue
		}
		if entry.info.IsDir() && !options.directoryItself {
			directories = append(directories, entry)
		} else {
			files = append(files, entry)
		}
	}
	written := len(files) > 0
	if written {
		if err := writeLsOperands(stdout, files, options); err != nil {
			return false, err
		}
	}
	sortLsEntries(directories, options)
	headed := len(targets) > 1 || options.recursive
	for _, directory := range directories {
		if written {
			if _, err := fmt.Fprintln(stdout); err != nil {
				return false, err
			}
		}
		written = true
		var err error
		if directory.device {
			// `/dev` itself, which is listed from the view rather than the disk. See
			// docs/design/device-filesystem.md for why this lists at all when the reference
			// does not.
			if headed {
				fmt.Fprintf(stdout, "%s:\n", directory.name)
			}
			err = listDeviceDirectory(stdout, view, directory.path, options)
		} else {
			err = listDirectory(stdout, directory.path, directory.name, options, headed)
		}
		if err != nil {
			if len(targets) == 1 || !isOperandFailure(err) || !reportOperand(ctx, err) {
				return false, err
			}
			listed = false
		}
	}
	return listed, nil
}

// statLsOperand is what one operand names. A device is described from the table rather than
// resolved to a host path it has not got: `ls -l /dev/null` answered "is not a host path"
// before that, where busybox prints a character device.
func statLsOperand(view ProcessView, target string) (lsEntry, error) {
	if info, err := statDeviceOperand(view, target); err != nil {
		return lsEntry{}, err
	} else if info != nil {
		return lsEntry{name: target, info: info, path: target, device: info.IsDir()}, nil
	}
	native, err := resolveHostPath(view, target)
	if err != nil {
		return lsEntry{}, err
	}
	info, err := os.Stat(native)
	if err != nil {
		return lsEntry{}, operandFailure(target, err)
	}
	return lsEntry{name: target, info: info, path: native}, nil
}

// writeLsOperands lays out the operands that are not directories: sorted, and in columns as a
// directory's entries are, but with no `total` line, which heads a directory's listing only.
func writeLsOperands(stdout io.Writer, items []lsEntry, options lsOptions) error {
	sortLsEntries(items, options)
	if !options.long {
		return writeLsNames(stdout, items, options, lsWantsColumns(options, stdout))
	}
	for _, item := range items {
		if err := printLsEntry(stdout, item, options); err != nil {
			return err
		}
	}
	return nil
}
