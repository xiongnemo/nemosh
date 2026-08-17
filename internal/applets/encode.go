package applets

import (
	"bufio"
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
		options, paths, err := parseAppletOptions(args, "di", "w")
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
	encoded := base64.StdEncoding.EncodeToString(data)
	if width == 0 {
		// No newline at all with -w0, matching GNU: `base64 -w0 | wc -l` is 0.
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

// The checksum pair. One implementation, two names, because the only difference
// is which hash is constructed -- and two copies of the -c parser would drift.
func newSha256sumApplet() Applet { return newChecksumApplet("sha256sum", sha256.New) }

func newMd5sumApplet() Applet { return newChecksumApplet("md5sum", md5.New) }

// The output format is GNU's `<hex>  <name>`, two spaces.
//
// Measured: the coreutils build on this machine prints `<hex> *<name>` instead,
// because it defaults to binary mode on Windows and marks it with the asterisk.
// Two spaces is chosen anyway -- it is what GNU prints on every other platform,
// what every README shows, and what a script comparing against a published
// checksum will have. Nothing is lost by it: this never translates line endings,
// so the two modes would produce identical digests here regardless of the mark.
//
// `-c` accepts either form for exactly that reason, since a file of sums may well
// have been produced by the build that writes the asterisk.
func newChecksumApplet(name string, newHash func() hash.Hash) Applet {
	return simpleApplet{name: name, runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		options, paths, err := parseAppletOptions(args, "bctw", "")
		if err != nil {
			return err
		}
		if options.has('c') {
			return checkSums(ctx, name, newHash, paths, stdin, stdout, stderr, options.has('w'))
		}
		if len(paths) == 0 {
			return writeSum(stdout, newHash, stdin, "-")
		}
		view := ProcessViewFromContext(ctx)
		for _, path := range paths {
			file, err := OpenProcessInput(ctx, view, path)
			if err != nil {
				return cannotOpen(path, err)
			}
			sumErr := writeSum(stdout, newHash, file, path)
			file.Close()
			if sumErr != nil {
				return sumErr
			}
		}
		return nil
	}}
}

func writeSum(stdout io.Writer, newHash func() hash.Hash, reader io.Reader, name string) error {
	digest := newHash()
	if _, err := io.Copy(digest, reader); err != nil {
		return err
	}
	_, err := fmt.Fprintf(stdout, "%s  %s\n", hex.EncodeToString(digest.Sum(nil)), name)
	return err
}

// checkSums verifies a list, printing `name: OK` or `name: FAILED` per line and
// failing overall if any did.
//
// A line that cannot be parsed is reported rather than skipped in silence. GNU
// warns and continues; the status is what a script reads, and a file of sums that
// is half garbage should not come back clean.
func checkSums(ctx context.Context, applet string, newHash func() hash.Hash, paths []string, stdin io.Reader, stdout, stderr io.Writer, warn bool) error {
	view := ProcessViewFromContext(ctx)
	failures, malformed := 0, 0
	verify := func(reader io.Reader) error {
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
		for scanner.Scan() {
			line := strings.TrimRight(scanner.Text(), "\r")
			if strings.TrimSpace(line) == "" {
				continue
			}
			want, name, ok := parseSumLine(line)
			if !ok {
				malformed++
				if warn {
					fmt.Fprintf(stderr, "%s: improperly formatted checksum line\n", applet)
				}
				continue
			}
			file, err := OpenProcessInput(ctx, view, name)
			if err != nil {
				failures++
				fmt.Fprintf(stdout, "%s: FAILED open or read\n", name)
				continue
			}
			digest := newHash()
			_, copyErr := io.Copy(digest, file)
			file.Close()
			if copyErr != nil {
				failures++
				fmt.Fprintf(stdout, "%s: FAILED open or read\n", name)
				continue
			}
			if hex.EncodeToString(digest.Sum(nil)) != want {
				failures++
				fmt.Fprintf(stdout, "%s: FAILED\n", name)
				continue
			}
			fmt.Fprintf(stdout, "%s: OK\n", name)
		}
		return scanner.Err()
	}
	if err := eachTextInput(ctx, paths, stdin, verify); err != nil {
		return err
	}
	switch {
	case failures > 0:
		return ExitStatusMessage(1, fmt.Errorf("%d computed checksum did NOT match", failures))
	case malformed > 0:
		return ExitStatusMessage(1, fmt.Errorf("no properly formatted checksum lines found"))
	}
	return nil
}

// parseSumLine reads `<hex>  <name>` and `<hex> *<name>`, which are GNU's text
// and binary spellings. Both are accepted because a list may have come from
// either build.
func parseSumLine(line string) (sum, name string, ok bool) {
	sum, rest, found := strings.Cut(line, " ")
	if !found || sum == "" {
		return "", "", false
	}
	if _, err := hex.DecodeString(sum); err != nil {
		return "", "", false
	}
	name = strings.TrimPrefix(rest, " ")
	name = strings.TrimPrefix(name, "*")
	if name == "" {
		return "", "", false
	}
	return sum, name, true
}
