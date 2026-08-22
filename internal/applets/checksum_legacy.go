package applets

import (
	"context"
	"fmt"
	"io"
)

// cksum, crc32 and sum: the three checksum tools that are not cryptographic
// hashes and do not share the `<hex>  <name>` format.
//
// They are grouped because each needs the file's *length* as well as its
// contents, which the hash family never does, and because each prints a
// different shape.

func newCksumApplet() Applet {
	return newSizedChecksumApplet("cksum", func(digest sizedDigest, name string, _ int) string {
		// `<crc> <size> <name>`, single spaces, and no name at all for stdin --
		// measured against busybox, which prints `3015617425 6` there.
		if name == "" {
			return fmt.Sprintf("%d %d", digest.posixCRC(), digest.size)
		}
		return fmt.Sprintf("%d %d %s", digest.posixCRC(), digest.size, name)
	})
}

func newCrc32Applet() Applet {
	return newSizedChecksumApplet("crc32", func(digest sizedDigest, name string, _ int) string {
		// Eight lowercase hex digits, and the ordinary IEEE polynomial rather
		// than cksum's. The two answer differently for the same bytes, which is
		// why both exist.
		if name == "" {
			return fmt.Sprintf("%08x", digest.ieeeCRC())
		}
		return fmt.Sprintf("%08x %s", digest.ieeeCRC(), name)
	})
}

// newSumApplet is the historical 16-bit checksum, in two incompatible flavours.
//
// -r is BSD and the default: the accumulator is rotated right before each byte
// is added, and the block count is in units of 1024. -s is System V: a plain
// byte sum folded twice into 16 bits, counted in 512-byte blocks. They disagree
// on the same file, which is the whole reason the option exists.
func newSumApplet() Applet {
	return simpleApplet{name: "sum", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(args, "rs", "")
		if err != nil {
			return err
		}
		systemV := options.has('s')
		return eachSizedChecksum(ctx, paths, stdin, stdout, func(digest sizedDigest, name string, operands int) string {
			if systemV {
				// System V prints the name even for one operand.
				return fmt.Sprintf("%d %d %s", digest.systemVSum(), blocksOf(digest.size, 512), name)
			}
			// BSD omits the name unless there is more than one operand, which is
			// what GNU does. busybox prints the format's trailing space with an
			// empty name instead -- `36979     1 ` -- a stray byte rather than a
			// behaviour, so this follows GNU here.
			line := fmt.Sprintf("%05d %5d", digest.bsdSum(), blocksOf(digest.size, 1024))
			if operands > 1 && name != "" {
				line += " " + name
			}
			return line
		})
	}}
}

// blocksOf is the file size in blocks, rounded up. An empty file is zero blocks
// rather than one, which both references agree on.
func blocksOf(size, block int64) int64 { return (size + block - 1) / block }

// newSizedChecksumApplet is the shape cksum and crc32 share: no options, one line
// per operand, and the name omitted when the input was stdin.
func newSizedChecksumApplet(name string, format sizedChecksumFormat) Applet {
	return simpleApplet{name: name, runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		_, paths, err := parseAppletOptions(args, "", "")
		if err != nil {
			return err
		}
		return eachSizedChecksum(ctx, paths, stdin, stdout, format)
	}}
}

// sizedChecksumFormat renders one line. operands is how many were named, which
// only `sum` needs -- it decides whether to print the name from it.
type sizedChecksumFormat func(digest sizedDigest, name string, operands int) string

// eachSizedChecksum reads every operand once and writes a line for it.
//
// With no operands the input is stdin and the name is empty, which is how each
// format knows to leave it out.
func eachSizedChecksum(ctx context.Context, paths []string, stdin io.Reader, stdout io.Writer, format sizedChecksumFormat) error {
	if len(paths) == 0 {
		digest, err := readSizedDigest(stdin)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, format(digest, "", 0))
		return err
	}
	view := ProcessViewFromContext(ctx)
	for _, path := range paths {
		file, err := OpenProcessOperand(ctx, view, path, stdin)
		if err != nil {
			return cannotOpen(path, err)
		}
		digest, readErr := readSizedDigest(file)
		closeErr := file.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		// A lone `-` names stdin, which has no filename to print.
		name := path
		if path == "-" {
			name = ""
		}
		if _, err := fmt.Fprintln(stdout, format(digest, name, len(paths))); err != nil {
			return err
		}
	}
	return nil
}
