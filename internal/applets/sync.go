package applets

import (
	"context"
	"fmt"
	"io"
	"os"
)

// sync is busybox's plain one (coreutils/sync.c without FEATURE_SYNC_FANCY): it writes every
// buffered block to disk, and ignores its arguments, saying so, as busybox does. On Windows that
// is busybox-w32's mingw_sync, which flushes each volume it can open, and opening one takes an
// elevated session; otherwise nothing is flushed, and nothing is said.
func newSyncApplet() Applet {
	return simpleApplet{name: "sync", run: func(args []string, _ io.Reader, _, stderr io.Writer) error {
		if len(args) > 0 {
			fmt.Fprintln(stderr, "sync: ignoring all arguments")
		}
		syncFilesystems()
		return nil
	}}
}

// fsync is busybox's (coreutils/sync.c): `fsync [-d] FILE...` writes each FILE's buffered blocks
// to disk, and -d its data alone, which is the same here. A FILE that cannot be opened is named
// and the rest are flushed. On Windows a handle opened only to read cannot be flushed, and that
// refusal is success, as busybox-w32's fsync has it, so a FILE that cannot be written is opened
// to read and passes.
func newFsyncApplet() Applet {
	return simpleApplet{name: "fsync", runContext: func(ctx context.Context, args []string, _ io.Reader, _, stderr io.Writer) error {
		_, operands, err := parseAppletOptions(ctx, args, "d", "")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return missingOperand()
		}
		view := ProcessViewFromContext(ctx)
		failed := false
		for _, operand := range operands {
			if err := fsyncOne(view, operand); err != nil {
				fmt.Fprintf(stderr, "fsync: %v\n", err)
				failed = true
			}
		}
		if failed {
			return ExitStatus(1)
		}
		return nil
	}}
}

// fsyncOne flushes one FILE: opened to write where it may be, so the flush reaches the disk, and
// otherwise to read, as busybox opens it.
func fsyncOne(view ProcessView, operand string) error {
	native, err := resolveHostPath(view, operand)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(native, os.O_WRONLY, 0)
	if err != nil {
		if file, err = os.Open(native); err != nil {
			return cannotOpen(operand, err)
		}
	}
	defer file.Close()
	if err := file.Sync(); err != nil && !isFlushRefusal(err) {
		return operandFailure(operand, err)
	}
	return nil
}
