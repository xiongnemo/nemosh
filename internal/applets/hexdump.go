package applets

import (
	"bufio"
	"context"
	"io"
	"math"
	"strings"
)

// hexdump is busybox's (util-linux/hexdump.c, over libbb/dump.c): `hexdump [-bcdoxCv] [-e FMT]
// [-f FMT_FILE] [-n LEN] [-s OFS] [FILE]...`, and hd is hexdump -C. Every FILE is one stream.
// Each of -b -c -d -o -x -C adds its format, in the order given, so `hexdump -C -C` prints each
// line twice, and -e adds FMT and -f each line of FMT_FILE; with none it is hexdump's words in
// hex. -s skips OFS bytes and -n reads no more than LEN, each a C number with K M G after it.
// A block the same as the one before is printed once and then `*`, unless -v.
//
// It printed fixed formats, one each, with no -e -f -n or -s: -x was the words of no option
// and -b -c -d -o were od's.
func newHexdumpApplet() Applet { return newLibbbDumpApplet("hexdump") }

func newHdApplet() Applet { return newLibbbDumpApplet("hd") }

func newLibbbDumpApplet(name string) Applet {
	return simpleApplet{name: name, runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(ctx, args, "bcdoxCv", "efns")
		if err != nil {
			return err
		}
		d := &dumper{length: -1}
		if name == "hd" {
			d.addCanonical()
		}
		taken := map[byte]int{}
		for _, letter := range options.order {
			value := ""
			if strings.IndexByte("efns", letter) >= 0 {
				value = options.all(letter)[taken[letter]]
				taken[letter]++
			}
			if err := d.option(ctx, letter, value); err != nil {
				return err
			}
		}
		if len(d.formats) == 0 {
			d.addWords(`8/2 " %04x`)
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
		return nil
	}}
}

// option is one of hexdump's options, taken in the order given, as busybox's getopt loop takes
// them: a format is read when its option is, and a bad one ends the command there.
func (d *dumper) option(ctx context.Context, letter byte, value string) error {
	var err error
	switch letter {
	case 'b', 'c', 'd', 'o', 'x':
		err = d.addWords(map[byte]string{'b': `16/1 " %03o`, 'c': `16/1 " %3_c`, 'd': `8/2 "   %05u`,
			'o': `8/2 "  %06o`, 'x': `8/2 "    %04x`}[letter])
	case 'C':
		err = d.addCanonical()
	case 'e':
		err = d.add(value)
	case 'f':
		err = d.addFile(ctx, value)
	case 'n':
		var length uint64
		length, err = busyboxNumber(value, math.MaxUint32, math.MaxInt32, kmgSuffixes)
		d.length = int64(length)
	case 's':
		var skip uint64
		skip, err = busyboxNumber(value, math.MaxUint64, math.MaxInt64, kmgSuffixes)
		d.skip = int64(skip)
	case 'v':
		d.verbose = true
	}
	return err
}

func (d *dumper) add(format string) error {
	parsed, err := parseDumpFormat(format)
	if err == nil {
		d.formats = append(d.formats, parsed)
	}
	return err
}

// addWords is busybox's add_format: the address in seven hex digits and then the words, and
// the length after the last block.
func (d *dumper) addWords(words string) error {
	if err := d.add("\"%07_Ax\n\""); err != nil {
		return err
	}
	return d.add("\"%07_ax\"" + words + "\"\"\n\"")
}

// addCanonical is -C: the address in eight hex digits, sixteen bytes in two groups of eight,
// and the text between bars.
func (d *dumper) addCanonical() error {
	for _, format := range []string{"\"%08_Ax\n\"", `"%08_ax "8/1 " %02x"" "8/1 " %02x"`, "\"  |\"16/1 \"%_p\"\"|\n\""} {
		if err := d.add(format); err != nil {
			return err
		}
	}
	return nil
}

// addFile is -f: each line of FILE a format, but for the blank ones and those that begin with
// a #.
func (d *dumper) addFile(ctx context.Context, path string) error {
	file, err := OpenProcessInput(ctx, ProcessViewFromContext(ctx), path)
	if err != nil {
		return cannotOpen(path, err)
	}
	defer file.Close()
	lines := bufio.NewScanner(file)
	lines.Buffer(nil, 1<<20)
	for lines.Scan() {
		line := strings.TrimLeft(lines.Text(), " \t\n\v\f\r")
		if line == "" || line[0] == '#' {
			continue
		}
		if err := d.add(line); err != nil {
			return err
		}
	}
	return lines.Err()
}
