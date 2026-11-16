package applets

import (
	"context"
	"errors"
	"io"
	"strings"
)

// iconv is busybox-w32's (miscutils/iconv.c), which is win_iconv: the conversion is Windows's
// own, MultiByteToWideChar and WideCharToMultiByte a character at a time through UTF-16, and
// MLang for EUC-JP and ISO-2022-JP, so every code page Windows has can be named. It was
// golang.org/x/text's encodings, a megabyte of tables for some forty of them.
//
//   - A name is one of busybox's 358 aliases, whatever its case, CP123 or 123 for a code page
//     by number, or nothing, which is UTF-8, the code page busybox-w32's manifest gives it.
//     //IGNORE, //TRANSLIT and //NOCOMPAT may follow one, as win_iconv reads them.
//   - A character the target cannot hold, or input that is no character, is `conversion
//     error: Illegal byte sequence`, after what came before it is written; -c passes over
//     both, as busybox's appends //IGNORE.
//   - A byte-order mark is read and written only for the names that ask for one, UTF-16 and
//     UTF-32 and their UCS spellings.
//   - -l lists the aliases in busybox's order.
//
// busybox opens its first operand again for every one after it; each is read here.

func newIconvApplet() Applet {
	return simpleApplet{name: "iconv", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(ctx, args, "lc", "fto")
		if err != nil {
			return err
		}
		if options.has('l') {
			return writeIconvNames(stdout)
		}
		tocode := options.value('t')
		if options.has('c') {
			tocode += "//IGNORE"
		}
		from, fromOK := newIconvCharset(options.value('f'))
		to, toOK := newIconvCharset(tocode)
		if !fromOK || !toOK {
			return errors.New("iconv_open error: Invalid argument")
		}
		converter := &iconvConverter{from: from, to: to}
		out := stdout
		if options.has('o') {
			file, err := createIconvOutput(ctx, options.value('o'))
			if err != nil {
				return err
			}
			defer file.Close()
			out = file
		}
		return eachTextInputQuoted(ctx, paths, stdin, func(reader io.Reader) error {
			return converter.convertStream(out, reader)
		})
	}}
}

func createIconvOutput(ctx context.Context, path string) (io.WriteCloser, error) {
	view := ProcessViewFromContext(ctx)
	native, err := resolveHostPath(view, path)
	if err != nil {
		return nil, operandFailure(path, err)
	}
	file, err := createFile(view, native)
	if err != nil {
		return nil, operandFailure(path, err)
	}
	return file, nil
}

// writeIconvNames is -l: every alias, in busybox's order.
func writeIconvNames(stdout io.Writer) error {
	_, err := io.WriteString(stdout, strings.ReplaceAll(iconvAliasNames, "\x00", "\n"))
	return err
}
