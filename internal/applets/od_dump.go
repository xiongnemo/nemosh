package applets

import (
	"context"
	"errors"
	"io"
	"math"
	"strings"
)

// od is busybox's (coreutils/od_bloaty.c): `od [-abcdfhilovxs] [-t TYPE] [-A RADIX] [-N SIZE]
// [-j SKIP] [-S MINSTR] [-w[WIDTH]] [FILE]...`, with --address-radix, --format, --read-bytes,
// --skip-bytes, --output-duplicates, --strings[=N] and --width[=N] for them, and --traditional
// for `od [FILE] [[+]OFFSET[.][b] [[+]LABEL[.][b]]]`. Every FILE is one stream, its offsets
// running on, and each -t TYPE a line of its own under the address: a kind, d o u x f a c, and
// a size, a count of bytes or C S I L F D. The letters are types too, and are taken in
// busybox's order whatever order they come in: -a, -b oC, -c, -d u2, -f fF, -h and -x x2, -i
// dI, -l dL, -o o2, the -t TYPEs, -s d2. With none it is -t o2. -j skips SKIP bytes and -N
// reads no more than SIZE, each C's number with b k m after it; -w puts WIDTH bytes on a line,
// 32 when -w has no number. -S prints the runs of MINSTR or more printable characters that end
// in a NUL, where each begins. A block the same as the one before is printed once and then
// `*`, unless -v.
//
// It took -b -c -d -o -x -v and a few -t TYPEs: -b and -d printed octal words, -t x one byte
// where it is four, -t o two, and -N -j -w -S -a -f -i -l -s were refused.
func newOdApplet() Applet {
	return simpleApplet{name: "od", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		words, traditional := odWords(args)
		options, paths, err := parseAppletLongOptions(ctx, words, map[string]string{
			"skip-bytes": "j", "address-radix": "A", "read-bytes": "N", "format": "t",
			"output-duplicates": "v", "strings": "S", "width": "w",
		}, "abcdDfhHiIlLoOBvxXs", "ANjtSw")
		if err != nil {
			return err
		}
		run, err := newOdRun(options)
		if err != nil {
			return err
		}
		if traditional {
			if paths, err = run.traditional(paths); err != nil {
				return err
			}
		}
		// Both are C's off_t, so a SIZE past it is less than none.
		if run.limited && run.skip+run.limit < run.skip {
			return errors.New("SKIP + SIZE is too large")
		}
		inputs := &dumpInputs{ctx: ctx, view: ProcessViewFromContext(ctx), stdin: stdin, paths: paths,
			exact: run.limited && !run.stringsGiven}
		if len(paths) == 0 {
			inputs.paths = []string{"-"}
		}
		defer inputs.Close()
		if err := run.dump(stdout, stderr, inputs); err != nil {
			return err
		}
		if inputs.failed {
			return ExitStatus(1)
		}
		return nil
	}}
}

// odWords gives -w and --width 32 where no WIDTH follows in the same word, and --strings 3, as
// getopt's optional arguments have it: `-w 8` is -w and a FILE named 8. --traditional, which
// has no letter, is taken out and answered apart.
func odWords(args []string) ([]string, bool) {
	words, traditional := make([]string, 0, len(args)), false
	for index, arg := range args {
		switch {
		case arg == "--":
			return append(words, args[index:]...), traditional
		case arg == "--traditional":
			traditional = true
			continue
		case arg == "--width":
			arg = "--width=32"
		case arg == "--strings":
			arg = "--strings=3"
		case len(arg) > 1 && arg[0] == '-' && arg[1] != '-':
			for at := 1; at < len(arg); at++ {
				if strings.IndexByte("ANjtS", arg[at]) >= 0 {
					break
				}
				if arg[at] == 'w' {
					if at == len(arg)-1 {
						arg += "32"
					}
					break
				}
			}
		}
		words = append(words, arg)
	}
	return words, traditional
}

// odAddressing is how an address is printed, od_bloaty.c's format_address_std, _none, _paren
// and _label; the last two are --traditional's LABEL.
type odAddressing byte

const (
	odAddressStd odAddressing = iota
	odAddressNone
	odAddressParen
	odAddressLabel
)

type odRun struct {
	specs []odSpec
	// pads is each spec's blanks to share among a line's fields, so that every line is as wide
	// as the widest.
	pads       []int
	addressing odAddressing
	radix      byte
	pad        int
	// pseudo is what LABEL adds to an offset.
	pseudo int64
	// width is the bytes on a line, which -w gives when widthGiven.
	width      int
	widthGiven bool
	verbose    bool
	skip       int64
	limit      int64
	limited    bool
	// strings is -S's MINSTR, when stringsGiven.
	strings      int64
	stringsGiven bool
}

// newOdRun reads the options in the order busybox's od_main does, which is whose error a
// command line with two bad ones gets: -w as getopt reads it, then -A, -N, -j, the TYPEs, -S.
func newOdRun(options appletOptions) (*odRun, error) {
	run := &odRun{radix: 'o', pad: 7, verbose: options.has('v'), widthGiven: options.has('w')}
	if run.widthGiven {
		width, err := positiveNumber(options.value('w'))
		if err != nil {
			return nil, err
		}
		run.width = width
	}
	if options.has('A') {
		// busybox reads the first character alone: -A xyz is hex.
		first, index := options.value('A'), -1
		if first != "" {
			first, index = first[:1], strings.IndexByte("doxn", first[0])
		}
		if index < 0 {
			return nil, errors.New("bad output address radix '" + first + "' (must be [doxn])")
		}
		run.radix, run.pad = "uoxn"[index], [...]int{7, 7, 6, 0}[index]
		if run.radix == 'n' {
			run.addressing = odAddressNone
		}
	}
	for _, number := range []struct {
		letter byte
		into   *int64
	}{{'N', &run.limit}, {'j', &run.skip}} {
		if options.has(number.letter) {
			value, err := busyboxNumber(options.value(number.letter), math.MaxUint64, math.MaxUint64, bkmSuffixes)
			if err != nil {
				return nil, err
			}
			*number.into = int64(value)
		}
	}
	run.limited = options.has('N')
	if err := run.types(options); err != nil {
		return nil, err
	}
	if run.stringsGiven = options.has('S'); run.stringsGiven {
		value, err := busyboxNumber(options.value('S'), math.MaxUint32, math.MaxUint32, bkmSuffixes)
		if err != nil {
			return nil, err
		}
		run.strings = int64(value)
	}
	return run, nil
}

// types is the specs the letters and -t name, in busybox's order.
func (r *odRun) types(options appletOptions) error {
	letters := []struct {
		given bool
		types string
	}{
		{options.has('a'), "a"}, {options.has('b'), "oC"}, {options.has('c'), "c"},
		{options.has('d'), "u2"}, {options.has('D'), "uI"}, {options.has('f'), "fF"},
		{options.has('h') || options.has('x'), "x2"}, {options.has('H') || options.has('X'), "xI"},
		// Where a long is an int, busybox-w32's -i -I -l -L are all the one int.
		{options.has('i') || odLongSize == 4 && (options.has('I') || options.has('l') || options.has('L')), "dI"},
		{odLongSize == 8 && (options.has('I') || options.has('l') || options.has('L')), "dL"},
		{options.has('o') || options.has('B'), "o2"}, {options.has('O'), "oI"},
	}
	for _, letter := range letters {
		if letter.given {
			specs, _ := parseOdTypes(letter.types)
			r.specs = append(r.specs, specs...)
		}
	}
	for _, types := range options.all('t') {
		specs, err := parseOdTypes(types)
		if err != nil {
			return err
		}
		r.specs = append(r.specs, specs...)
	}
	if options.has('s') {
		specs, _ := parseOdTypes("d2")
		r.specs = append(r.specs, specs...)
	}
	if len(r.specs) == 0 {
		r.specs, _ = parseOdTypes("o2")
	}
	return nil
}

// lcm is the least common multiple of the specs' sizes: a line is a whole number of each.
func (r *odRun) lcm() int {
	lcm := 1
	for _, spec := range r.specs {
		a, b := lcm, spec.size
		for b != 0 {
			a, b = b, a%b
		}
		lcm = lcm / a * spec.size
	}
	return lcm
}

// columns stands several TYPEs' lines in columns, `-t c -t x1` putting each byte's number
// under its character, as GNU od and the scripts written for it have them; busybox's od leaves
// each line as narrow as its own fields.
func (r *odRun) columns() {
	widest := 0
	for _, spec := range r.specs {
		widest = max(widest, r.width/spec.size*(spec.width+1))
	}
	r.pads = make([]int, len(r.specs))
	for index, spec := range r.specs {
		r.pads[index] = widest - r.width/spec.size*(spec.width+1)
	}
}
