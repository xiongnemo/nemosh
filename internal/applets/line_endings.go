package applets

import (
	"bytes"
	"context"
	"io"
	"os"
)

// dos2unix and unix2dos: one implementation, two names, the direction chosen by
// the name it was invoked as -- and overridable with -u and -d, which is exactly
// what busybox offers.
//
// The Windows-specific tool on the list, and the one whose default is a trap: a
// file operand is converted **in place**, with no output to stdout at all. With
// no operand it filters stdin to stdout. Measured against busybox-w32 on
// 2026-08-22.

func newDos2unixApplet() Applet { return newLineEndingApplet("dos2unix", toUnixEndings) }

func newUnix2dosApplet() Applet { return newLineEndingApplet("unix2dos", toDOSEndings) }

// lineEndingDirection converts a whole file's bytes.
type lineEndingDirection func([]byte) []byte

func newLineEndingApplet(name string, direction lineEndingDirection) Applet {
	return simpleApplet{name: name, runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(args, "ud", "")
		if err != nil {
			return err
		}
		// -u and -d name the direction outright, so `unix2dos -u` really does
		// convert to Unix endings. The last one given wins, since both write the
		// same setting.
		if options.has('u') {
			direction = toUnixEndings
		}
		if options.has('d') {
			direction = toDOSEndings
		}
		if len(paths) == 0 {
			data, err := io.ReadAll(stdin)
			if err != nil {
				return err
			}
			_, err = stdout.Write(direction(data))
			return err
		}
		return convertLineEndingsInPlace(ctx, paths, direction)
	}}
}

// convertLineEndingsInPlace rewrites each operand.
//
// The whole file is read before anything is written, for the same reason `sed -i`
// does it: the output is derived from the input, so streaming into the same path
// would overwrite bytes still waiting to be read. It also means a failure leaves
// the file as it was.
func convertLineEndingsInPlace(ctx context.Context, paths []string, direction lineEndingDirection) error {
	view := ProcessViewFromContext(ctx)
	for _, path := range paths {
		native, err := resolveHostPath(view, path)
		if err != nil {
			return operandFailure(path, err)
		}
		original, err := os.ReadFile(native)
		if err != nil {
			return operandFailure(path, err)
		}
		converted := direction(original)
		if bytes.Equal(converted, original) {
			// Nothing to do, and not rewritten: an unchanged file keeps its
			// modification time, so a build system is not told it changed.
			continue
		}
		mode := os.FileMode(0o644)
		if info, err := os.Stat(native); err == nil {
			mode = info.Mode().Perm()
		}
		if err := os.WriteFile(native, converted, mode); err != nil {
			return operandFailure(path, err)
		}
	}
	return nil
}

// toUnixEndings drops the CR of every CRLF.
//
// Only before a newline: a lone CR is left alone, because on a Mac Classic file
// it is the line ending itself and dropping it would join every line into one.
// A CR in binary data is left alone for the same reason -- this converts line
// endings, not carriage returns.
func toUnixEndings(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

// toDOSEndings gives every newline a CR, without doubling the ones that have one.
//
// Normalising to LF first is what makes this idempotent: `unix2dos` twice must
// not produce CRCRLF, which is what a bare LF-to-CRLF replacement does on its
// second run.
func toDOSEndings(data []byte) []byte {
	return bytes.ReplaceAll(toUnixEndings(data), []byte("\n"), []byte("\r\n"))
}
