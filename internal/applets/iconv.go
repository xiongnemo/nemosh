package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// iconv: convert text between character encodings.
//
// This is the applet that settles a question two others were waiting on.
// docs/support-matrix.md records `sed -i` and `wc -m` as outstanding on UTF-16
// input, and the reason was that neither had a policy for which encoding to
// *write*. iconv is the tool whose entire job is that choice, so the policy lives
// here and the others can follow it:
//
//   - An encoding is named, never guessed. -f and -t are explicit; there is no
//     detection step, for the same reason grep only honours a byte-order mark and
//     never sniffs (text_utf16.go). Guessing is how a binary gets rewritten.
//   - Both default to UTF-8, so `iconv file` is a no-op rather than a surprise.
//   - Output carries no byte-order mark unless the named encoding is one of the
//     explicit BOM forms. Adding one uninvited breaks `#!` lines and CSV headers.
//   - A character the target cannot represent is an error, not a silent
//     substitution, unless -c asks for it to be dropped.
//
// golang.org/x/text is already a dependency (go.mod), so this needs nothing new.
// ianaindex maps the names people actually type -- GBK, Shift_JIS, windows-1252 --
// onto codecs.

func newIconvApplet() Applet {
	return simpleApplet{name: "iconv", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(args, "lc", "fto")
		if err != nil {
			return err
		}
		if options.has('l') {
			return writeIconvNames(stdout)
		}
		from, err := lookupEncoding(options, 'f')
		if err != nil {
			return err
		}
		to, err := lookupEncoding(options, 't')
		if err != nil {
			return err
		}
		out := stdout
		if options.has('o') {
			file, err := createIconvOutput(ctx, options.value('o'))
			if err != nil {
				return err
			}
			defer file.Close()
			out = file
		}
		return eachTextInput(ctx, paths, stdin, func(reader io.Reader) error {
			return convertEncoding(out, reader, from, to, options.has('c'))
		})
	}}
}

// lookupEncoding resolves one -f or -t value, defaulting to UTF-8.
//
// The IANA index is consulted first because it knows the aliases people type;
// the explicit UTF-16 forms are handled before it because the index maps plain
// "UTF-16" to a BOM-sniffing decoder, which is exactly the guessing this refuses
// to do.
func lookupEncoding(options appletOptions, letter byte) (encoding.Encoding, error) {
	if !options.has(letter) {
		return unicode.UTF8, nil
	}
	name := strings.TrimSpace(options.value(letter))
	switch strings.ToUpper(strings.ReplaceAll(name, "-", "")) {
	case "UTF8", "CP65001":
		return unicode.UTF8, nil
	case "UTF16LE", "UCS2LE":
		return unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM), nil
	case "UTF16BE", "UCS2BE":
		return unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM), nil
	case "UTF16":
		// The one place a mark is honoured on input and written on output, because
		// the name itself is what asks for it.
		return unicode.UTF16(unicode.LittleEndian, unicode.UseBOM), nil
	}
	found, err := ianaindex.IANA.Encoding(name)
	if err != nil || found == nil {
		return nil, fmt.Errorf("conversion from/to `%s' is not supported", name)
	}
	return found, nil
}

func convertEncoding(stdout io.Writer, reader io.Reader, from, to encoding.Encoding, drop bool) error {
	decoder := from.NewDecoder()
	encoder := to.NewEncoder()
	if drop {
		// -c: a character the target cannot hold is dropped rather than failing
		// the whole conversion. Without it the failure is loud, which is the right
		// default -- silently losing characters is how text is corrupted.
		encoder = encoding.ReplaceUnsupported(to.NewEncoder())
	}
	converted := transform.NewReader(transform.NewReader(reader, decoder), encoder)
	_, err := io.Copy(stdout, converted)
	if err != nil {
		return fmt.Errorf("cannot convert: %v", err)
	}
	return nil
}

func createIconvOutput(ctx context.Context, path string) (io.WriteCloser, error) {
	native, err := resolveHostPath(ProcessViewFromContext(ctx), path)
	if err != nil {
		return nil, operandFailure(path, err)
	}
	file, err := os.Create(native)
	if err != nil {
		return nil, operandFailure(path, err)
	}
	return file, nil
}

// writeIconvNames lists what -f and -t accept.
//
// The list is the encodings this build can actually construct, not every name
// IANA knows: a name that appears here and then fails to convert would be worse
// than one that never appeared.
func writeIconvNames(stdout io.Writer) error {
	names := []string{"UTF-8", "UTF-16", "UTF-16LE", "UTF-16BE"}
	for _, candidate := range iconvCandidateNames {
		if found, err := ianaindex.IANA.Encoding(candidate); err == nil && found != nil {
			names = append(names, candidate)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if _, err := fmt.Fprintln(stdout, name); err != nil {
			return err
		}
	}
	return nil
}

// iconvCandidateNames are the encodings worth offering on a Windows machine: the
// code pages a file here is actually likely to be saved in.
var iconvCandidateNames = []string{
	"ISO-8859-1", "ISO-8859-2", "ISO-8859-5", "ISO-8859-7", "ISO-8859-15",
	"windows-1250", "windows-1251", "windows-1252", "windows-1253", "windows-1254",
	"windows-1255", "windows-1256", "windows-1257", "windows-1258",
	"KOI8-R", "KOI8-U", "IBM866", "macintosh",
	"Shift_JIS", "EUC-JP", "ISO-2022-JP",
	"EUC-KR", "GBK", "GB18030", "Big5", "HZ-GB-2312",
}
