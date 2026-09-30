package applets

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"strconv"
	"strings"
)

// base64 and the checksums: what you reach for after downloading something, and
// what a clean Windows machine cannot do at all.
//
// Measured against GNU coreutils, whose output format is the one every README
// tells people to compare against.

// base64Wrap is GNU's default line length. `-w0` turns wrapping off, which is
// what anyone piping the output into something else wants.
const base64Wrap = 76

func newBase64Applet() Applet {
	return simpleApplet{name: "base64", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(ctx, args, "di", "w")
		if err != nil {
			return err
		}
		width := base64Wrap
		if options.has('w') {
			value := options.value('w')
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 {
				return fmt.Errorf("invalid wrap size: %s", value)
			}
			width = parsed
		}
		if options.has('d') {
			return eachTextInput(ctx, paths, stdin, func(reader io.Reader) error {
				return decodeBase64(reader, stdout, options.has('i'))
			})
		}
		return eachTextInput(ctx, paths, stdin, func(reader io.Reader) error {
			return encodeBase64(reader, stdout, width)
		})
	}}
}

func encodeBase64(reader io.Reader, stdout io.Writer, width int) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	return writeWrapped(stdout, base64.StdEncoding.EncodeToString(data), width)
}

// writeWrapped emits text in lines of at most width characters.
//
// Shared with base32, which wraps identically. Width zero means no newline at
// all, matching GNU: `base64 -w0 | wc -l` is 0, and anyone piping the output
// onward is relying on that.
func writeWrapped(stdout io.Writer, encoded string, width int) error {
	if width == 0 {
		_, err := io.WriteString(stdout, encoded)
		return err
	}
	for start := 0; start < len(encoded); start += width {
		end := min(start+width, len(encoded))
		if _, err := io.WriteString(stdout, encoded[start:end]+"\n"); err != nil {
			return err
		}
	}
	return nil
}

// decodeBase64 ignores newlines, which is not politeness but necessity: the
// encoder wraps at 76 columns, so its own output is not valid base64 to a strict
// decoder. Measured that GNU accepts its own wrapped output.
//
// Other rubbish is refused unless -i asked for it to be skipped, because a
// truncated download that decodes to nearly the right bytes is worse than one
// that says it is broken.
func decodeBase64(reader io.Reader, stdout io.Writer, ignoreGarbage bool) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	var cleaned strings.Builder
	for _, r := range string(data) {
		switch {
		case r == '\n' || r == '\r':
			continue
		case ignoreGarbage && !isBase64Rune(r):
			continue
		}
		cleaned.WriteRune(r)
	}
	decoded, err := base64.StdEncoding.DecodeString(cleaned.String())
	if err != nil {
		return fmt.Errorf("invalid input")
	}
	_, err = stdout.Write(decoded)
	return err
}

func isBase64Rune(r rune) bool {
	switch {
	case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		return true
	}
	return r == '+' || r == '/' || r == '='
}

// The checksum family. One implementation, several names, because the only
// difference is which hash is constructed -- and several copies of the -c parser
// would drift. The rest of the names are in checksum_family.go.
func newSha256sumApplet() Applet { return newChecksumApplet("sha256sum", sha256.New) }

func newMd5sumApplet() Applet { return newChecksumApplet("md5sum", md5.New) }

// The output format is `<hex>  <name>`, two spaces, and with -b `<hex> *<name>`, busybox's and
// GNU's alike; -t after -b is two spaces again.
//
// Measured: the coreutils build on this machine prints `<hex> *<name>` without -b,
// because it defaults to binary mode on Windows and marks it with the asterisk.
// Two spaces is chosen anyway -- it is what busybox-w32 prints, what GNU prints on
// every other platform, what every README shows, and what a script comparing against
// a published checksum will have. Nothing is lost by it: this never translates line
// endings, so the two modes produce identical digests here whatever the mark.
//
// `-c` accepts either form for exactly that reason, since a file of sums may well
// have been produced by the build that writes the asterisk.
func newChecksumApplet(name string, newHash func() hash.Hash) Applet {
	return newChecksumAppletWith(name, "", func(appletOptions) (func() hash.Hash, error) {
		return newHash, nil
	})
}

// newChecksumAppletWith is the same applet for a name whose hash depends on an
// option: sha3sum picks its width with -a. valued lists the letters that take an
// argument, and resolve turns the parsed options into a hash.
func newChecksumAppletWith(name, valued string, resolve func(appletOptions) (func() hash.Hash, error)) Applet {
	return simpleApplet{name: name, runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		options, paths, err := parseAppletOptions(ctx, args, "bcstw", valued)
		if err != nil {
			return err
		}
		newHash, err := resolve(options)
		if err != nil {
			return err
		}
		if options.has('c') {
			return checkSums(ctx, name, newHash, paths, stdin, stdout, stderr, options.has('s'), options.has('w'))
		}
		// -s and -w say how -c reports, and busybox refuses either without it.
		for _, letter := range []byte{'s', 'w'} {
			if options.has(letter) {
				return fmt.Errorf("-%c requires -c", letter)
			}
		}
		mark := " "
		if options.last("bt") == 'b' {
			mark = "*"
		}
		if len(paths) == 0 {
			return writeSum(stdout, newHash, stdin, mark+"-")
		}
		view := ProcessViewFromContext(ctx)
		opened := true
		for _, path := range paths {
			file, err := OpenProcessOperand(ctx, view, path, stdin)
			if err != nil {
				// Of several operands, one that cannot be opened is named and the rest summed; see
				// operand_reporter.go.
				if len(paths) == 1 || !reportOperand(ctx, cannotOpen(path, err)) {
					return cannotOpen(path, err)
				}
				opened = false
				continue
			}
			sumErr := writeSum(stdout, newHash, file, mark+path)
			file.Close()
			if sumErr != nil {
				return sumErr
			}
		}
		if !opened {
			return ExitStatus(1)
		}
		return nil
	}}
}

// writeSum writes reader's hash and name, which comes marked: a blank for text, a `*` for binary.
func writeSum(stdout io.Writer, newHash func() hash.Hash, reader io.Reader, marked string) error {
	digest := newHash()
	if _, err := io.Copy(digest, reader); err != nil {
		return err
	}
	_, err := fmt.Fprintf(stdout, "%s %s\n", hex.EncodeToString(digest.Sum(nil)), marked)
	return err
}
