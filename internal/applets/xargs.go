package applets

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
)

// xargs is busybox's (findutils/xargs.c): `xargs [OPTIONS] [PROG ARGS]` runs PROG with ARGS
// and as many of the input's words as a command line holds, and again until the input ends;
// with no PROG it is echo. A blank ends a word, and quotes and a backslash keep one in it; -0
// ends it at a NUL alone, and -I STR or -i[STR] reads a line, put in place of STR in ARGS. -n
// N is no more than N words to a command line, -s N no more than N bytes of it, 30720 with
// none. -E STR or -e[STR] ends the input at a word STR; -r runs nothing for no words; -t
// writes each command line to stderr first and -p asks at the console first; -a FILE reads
// FILE rather than stdin; -P N runs up to N PROGs at once. -x is taken, and does nothing, as
// in busybox. A PROG that ends 1 to 254 makes xargs end 123 when the input is done, one that
// ends 255 stops it, 124, and one there is no applet for stops it, 127.
//
// PROG is an applet: this runs no program of the operating system's, as no applet does.
//
// It split at blanks alone, read the whole input before running anything, took -0 -r -t -n
// and -I and no others, and ended at a PROG's first failure with its status.
type xargsApplet struct{}

func newXargsApplet() Applet { return xargsApplet{} }

func (xargsApplet) Name() string { return "xargs" }

type xargsRun struct {
	ctx            context.Context
	stdout, stderr io.Writer
	reader         *xargsReader
	command        []string
	room, most     int
	nul            bool
	placeholder    string
	replacing      bool
	noEmpty        bool
	verbose        bool
	confirm        bool
	pool           *xargsPool
	// failed is whether a PROG ended with a status of 1 to 254.
	failed bool
	mutex  sync.Mutex
}

func (xargsApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	options, command, err := parseAppletOptionsInOrder(xargsWords(args), "trpx0", "nsEIPa")
	if err != nil {
		return err
	}
	run := &xargsRun{ctx: ctx, stdout: stdout, stderr: stderr, command: command, room: 32*1024 - 2048,
		nul: options.has('0'), noEmpty: options.has('r'), verbose: options.has('t'), confirm: options.has('p')}
	if len(run.command) == 0 {
		run.command = []string{"echo"}
	}
	if options.has('P') {
		procs, err := positiveNumber(options.value('P'))
		if err != nil {
			return err
		}
		if procs != 1 {
			lock := &sync.Mutex{}
			run.pool, run.stdout, run.stderr = newXargsPool(procs), lockedWriter{lock, stdout}, lockedWriter{lock, stderr}
		}
	}
	input := stdin
	if options.has('a') {
		file, err := OpenProcessInput(ctx, ProcessViewFromContext(ctx), options.value('a'))
		if err != nil {
			return cannotOpen(options.value('a'), err)
		}
		defer file.Close()
		input = file
	}
	run.reader = &xargsReader{input: bufio.NewReader(input)}
	if eof := options.value('E'); options.has('E') && eof != "" {
		run.reader.eof = &eof
	}
	if options.has('s') {
		if run.room, err = xargsCount(options.value('s')); err != nil {
			return err
		}
	}
	for _, word := range run.command {
		run.room -= len(word) + 1
	}
	if run.room <= 0 {
		return errors.New("cannot fit single argument within argument list size limit")
	}
	run.most = run.room
	if options.has('n') {
		if run.most, err = xargsCount(options.value('n')); err != nil {
			return err
		}
	}
	if run.placeholder, run.replacing = options.value('I'), options.has('I'); run.replacing {
		// busybox's corrupts its heap on an empty STR, which would be put between every two
		// characters of each ARG.
		if run.placeholder == "" {
			return errors.New("replacement string cannot be empty")
		}
		run.noEmpty = true
	}
	return run.loop()
}

// xargsWords reads -e[STR] and -i[STR] as -E STR and -I STR, getopt's optional arguments
// being in their own word or not at all, and --no-run-if-empty as -r. The options end at the
// first word that is not one: PROG's own are its.
func xargsWords(args []string) []string {
	words := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--no-run-if-empty":
			words = append(words, "-r")
			continue
		case arg == "--" || len(arg) < 2 || arg[0] != '-' || strings.HasPrefix(arg, "--"):
			return append(words, args[index:]...)
		}
		for at := 1; at < len(arg); at++ {
			switch letter := arg[at]; {
			case letter == 'e' || letter == 'i':
				value := arg[at+1:]
				if letter == 'i' && value == "" {
					value = "{}"
				}
				words, at = append(words, "-"+strings.ToUpper(string(letter)), value), len(arg)
			case strings.IndexByte("nsEIPa", letter) >= 0:
				words = append(words, "-"+arg[at:])
				if at == len(arg)-1 && index+1 < len(args) {
					index++
					words = append(words, args[index])
				}
				at = len(arg)
			default:
				words = append(words, "-"+string(letter))
			}
		}
	}
	return words
}

// xargsCount is -n's and -s's N, busybox's xatou_range from 1 to INT_MAX.
func xargsCount(text string) (int, error) {
	value, err := strconv.ParseUint(text, 10, 32)
	switch {
	case err != nil || text == "" || text[0] == '+':
		return 0, fmt.Errorf("invalid number '%s'", text)
	case value < 1 || value > math.MaxInt32:
		return 0, fmt.Errorf("number %s is not in 1..%d range", text, math.MaxInt32)
	}
	return int(value), nil
}

// loop is xargs_main's: a command line from each turn's words, the first run with none unless
// -r, and none after it; and then the end of the PROGs still running.
func (x *xargsRun) loop() error {
	err := x.feed()
	if x.pool != nil {
		if stop := x.pool.wait(); err == nil {
			err = stop
		}
	}
	if err == nil && x.failed {
		err = ExitStatus(123)
	}
	return err
}

func (x *xargsRun) feed() error {
	for {
		line, ok, err := x.next()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		x.noEmpty = true
		if x.verbose || x.confirm {
			fmt.Fprint(x.stderr, strings.Join(line, " "))
			if !x.confirm {
				fmt.Fprintln(x.stderr)
			}
		}
		if x.confirm && !x.ask() {
			continue
		}
		if err := x.exec(line); err != nil {
			return err
		}
	}
	return nil
}

// next is the next command line, and false when there is none to run.
func (x *xargsRun) next() ([]string, bool, error) {
	if x.replacing {
		end := map[bool]byte{true: 0, false: '\n'}[x.nul]
		line, fitted, err := x.reader.line(x.room, end)
		switch {
		case err != nil:
			return nil, false, err
		case !fitted:
			return nil, false, errors.New("argument line too long")
		}
		return replaced(x.command, x.placeholder, line), line != "", nil
	}
	read := x.reader.words
	if x.nul {
		read = x.reader.nulWords
	}
	words, err := read(x.room, x.most)
	if err != nil {
		return nil, false, err
	}
	if len(words) == 0 {
		if len(x.reader.pending) > 0 {
			return nil, false, errors.New("argument line too long")
		}
		if x.noEmpty {
			return nil, false, nil
		}
	}
	return append(append([]string(nil), x.command...), words...), true, nil
}

// exec runs one command line, and says as busybox's xargs_exec does whether to stop: at a PROG
// there is no applet for, 127, and at one that ends 255, 124.
func (x *xargsRun) exec(line []string) error {
	applet, ok := commandFor(x.ctx, line[0])
	if !ok {
		return ExitStatusMessage(127, fmt.Errorf("%s: No such file or directory", line[0]))
	}
	if x.pool != nil {
		return x.pool.start(x, applet, line)
	}
	// Nothing is fed to PROG's stdin: xargs has read it to build the command line, and handing
	// on what is left would have `xargs cat` read its own input.
	return x.settle(line[0], applet.Run(x.ctx, line[1:], bytes.NewReader(nil), x.stdout, x.stderr))
}

// xargsStatus is the status PROG's error stands for, its diagnostic written as the shell
// writes one: a status of its own, 1 for false and for any other failure.
func xargsStatus(name string, err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	if status, ok := StatusCode(err); ok {
		if message, ok := StatusMessage(err); ok {
			fmt.Fprintf(stderr, "%s: %s\n", name, message)
		}
		return status
	}
	if !errors.Is(err, ErrExitFalse) {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
	}
	return 1
}
