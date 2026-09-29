package applets

import (
	"context"
	"fmt"
	"io"
	"math"
	"strings"
)

// xxd is busybox's (util-linux/hexdump_xxd.c): `xxd [-ri] [-ps] [-g N] [-c N] [-l LEN] [-s OFS]
// [-o OFS] [FILE]`, a dump of FILE or stdin through libbb's dump, in formats made from its
// options: the address, the bytes in groups of -g, and the text, -c bytes to a line; -p's bytes
// alone, 30 to a line; or -i's C. -l dumps no more than LEN bytes, -s skips OFS, and -o adds OFS
// to each address shown. -r is the reverse, bytes from such a dump.
//
// It printed its layout and -p's from code of its own, and took no other option; -p put every
// byte on one line.
func newXxdApplet() Applet {
	return simpleApplet{name: "xxd", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(ctx, xxdPlainArgument(args), "apir", "lsgco")
		if err != nil {
			return err
		}
		sizes := map[byte]int{}
		for _, letter := range []byte("gc") {
			for _, value := range options.all(letter) {
				if sizes[letter], err = positiveNumber(value); err != nil {
					return err
				}
			}
		}
		if len(paths) > 1 {
			return fmt.Errorf("extra operand '%s'", paths[1])
		}
		d := &dumper{length: -1, verbose: true}
		if err := d.xxdRange(options); err != nil {
			return err
		}
		if options.has('r') {
			return xxdReverse(ctx, paths, stdin, stdout, options.has('p'), d.skip)
		}
		if options.has('o') {
			if d.displayOffset, err = xxdSigned(options.value('o')); err != nil {
				return err
			}
		}
		group := 2
		if options.has('g') {
			group = sizes['g']
		}
		if err := d.addXxd(options.has('p'), options.has('i'), sizes['c'], group); err != nil {
			return err
		}
		return d.runXxd(ctx, paths, stdin, stdout, options.has('i') && len(paths) == 1)
	}}
}

// xxdPlainArgument is -p as busybox's getopt32 reads "p::": the rest of its word is its
// argument, which xxd ignores, so -ps is -p, and so is -pr. A value is left as it is.
func xxdPlainArgument(args []string) []string {
	out := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			return append(out, args[index:]...)
		}
		out = append(out, arg)
		if len(arg) < 2 || arg[0] != '-' {
			continue
		}
		for at := 1; at < len(arg); at++ {
			if strings.IndexByte("lsgco", arg[at]) >= 0 {
				if at == len(arg)-1 && index+1 < len(args) {
					index++
					out = append(out, args[index])
				}
				break
			}
			if arg[at] == 'p' {
				out[len(out)-1] = arg[:at+1]
				break
			}
		}
	}
	return out
}

// xxdRange is -l and -s, C numbers as xstrtou_range and xstrtoull_range read them.
func (d *dumper) xxdRange(options appletOptions) error {
	if options.has('l') {
		length, err := busyboxNumber(options.value('l'), math.MaxUint32, math.MaxInt32, nil)
		if err != nil {
			return err
		}
		d.length = int64(length)
	}
	if options.has('s') {
		skip, err := busyboxNumber(options.value('s'), math.MaxUint64, math.MaxInt64, nil)
		if err != nil {
			return err
		}
		d.skip = int64(skip)
	}
	return nil
}

// xxdSigned is xstrtoll in base 0: a C number with a sign before it, as -o takes one.
func xxdSigned(text string) (int64, error) {
	body, negative := text, false
	if body != "" && (body[0] == '+' || body[0] == '-') {
		body, negative = body[1:], body[0] == '-'
	}
	limit := uint64(math.MaxInt64)
	if negative {
		limit++
	}
	value, err := busyboxNumber(body, math.MaxUint64, limit, nil)
	if negative {
		return -int64(value), err
	}
	return int64(value), err
}

// addXxd is busybox's formats for xxd's options (util-linux/hexdump_xxd.c:280-334). A group of
// no bytes, or of a line's, is the line's bytes run together; each other group has a blank
// after it but the last. -p and -i end a short line where the input does, with no padding.
func (d *dumper) addXxd(plain, include bool, columns, group int) error {
	var formats []string
	switch {
	case plain:
		columns = orDefault(columns, 30)
		group = columns
	case include:
		columns, group = orDefault(columns, 12), 1
		formats = append(formats, `" "`)
	default:
		columns = orDefault(columns, 16)
		formats = append(formats, `"%08_ax: "`)
	}
	switch {
	case group < 1 || group >= columns:
		formats = append(formats, fmt.Sprintf(`%d/1 "%%02x"`, columns))
	case group == 1 && include:
		formats = append(formats, fmt.Sprintf(`%d/1 " 0x%%02x,"`, columns))
	case group == 1:
		formats = append(formats, fmt.Sprintf(`%d/1 "%%02x "`, columns))
	default:
		var bytes strings.Builder
		for index := 1; index <= columns; index++ {
			if index == columns || index%group != 0 {
				bytes.WriteString(`/1 "%02x"`)
			} else {
				bytes.WriteString(`/1 "%02x "`)
			}
		}
		formats = append(formats, bytes.String())
	}
	if plain || include {
		formats = append(formats, "\"\n\"")
		d.eofString = "\n"
	} else {
		formats = append(formats, fmt.Sprintf("\"  \"%d/1 \"%%_p\"\"\n\"", columns))
	}
	for _, format := range formats {
		if err := d.add(format); err != nil {
			return err
		}
	}
	return nil
}

// orDefault is value, or fallback when value is 0, as busybox's cols defaults.
func orDefault(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

// runXxd dumps FILE or stdin, with -i and a FILE as C: an array named for the FILE, and its
// length, which is where the dump ended. busybox prints the array's name before it opens the
// FILE, and so before it names one it cannot open.
func (d *dumper) runXxd(ctx context.Context, paths []string, stdin io.Reader, stdout io.Writer, include bool) error {
	if include {
		if _, err := fmt.Fprintf(stdout, "unsigned char %s[] = {\n", xxdCName(paths[0])); err != nil {
			return err
		}
	}
	inputs := &dumpInputs{ctx: ctx, view: ProcessViewFromContext(ctx), stdin: stdin, paths: paths, exact: d.length >= 0}
	if len(paths) == 0 {
		inputs.paths = []string{"-"}
	}
	defer inputs.Close()
	if err := d.run(stdout, inputs); err != nil {
		return err
	}
	if inputs.failed {
		return ExitStatus(1)
	}
	if include {
		_, err := fmt.Fprintf(stdout, "};\nunsigned int %s_len = %d;\n", xxdCName(paths[0]), d.address)
		return err
	}
	return nil
}

// xxdCName is busybox's print_C_style: the FILE's name as a C identifier, each byte that
// cannot be in one an underscore, and __ before a leading digit.
func xxdCName(name string) string {
	var out strings.Builder
	if name != "" && isASCIIDigit(name[0]) {
		out.WriteString("__")
	}
	for index := 0; index < len(name); index++ {
		c := name[index]
		if isASCIIDigit(c) || c|0x20 >= 'a' && c|0x20 <= 'z' {
			out.WriteByte(c)
		} else {
			out.WriteByte('_')
		}
	}
	return out.String()
}
