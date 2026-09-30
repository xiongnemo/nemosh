package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"time"
)

// touch is busybox's (coreutils/touch.c): `touch [-cham] [-d DATE] [-t DATE] [-r FILE] FILE...`
// sets each FILE's access and modification times, to now or to what -r's FILE has or to DATE,
// which -d and -t both read as parse_datestr reads it (datestr.go). -a sets only the access
// time and -m only the modification time; -c creates nothing that is not there; -h sets a
// symbolic link's own times rather than its target's. -f is accepted and ignored, and the long
// forms stand for their letters. A FILE that cannot be touched is named and the rest are, with
// status 1.
//
// touch took -c alone, so `touch -d 2020-01-01 stamp`, `touch -r ref copy` and `touch -t
// 202001010000 f` were refused as invalid options.
type touchApplet struct{}

func newTouchApplet() Applet { return touchApplet{} }

func (touchApplet) Name() string { return "touch" }

// touchLongOptions are busybox's long forms and the letter each stands for.
var touchLongOptions = map[string]string{"no-create": "c", "no-dereference": "h", "reference": "r", "date": "d"}

func (touchApplet) Run(ctx context.Context, args []string, _ io.Reader, _ io.Writer, stderr io.Writer) error {
	var words []string
	for index := 0; index < len(args); index++ {
		name, value, valued := strings.Cut(strings.TrimPrefix(args[index], "--"), "=")
		letter, long := touchLongOptions[name]
		switch {
		case args[index] == "--" || !strings.HasPrefix(args[index], "--"):
			words = append(words, args[index])
		case !long || valued && (letter == "c" || letter == "h"):
			return fmt.Errorf("unrecognized option '%s'", args[index])
		case valued:
			words = append(words, "-"+letter, value)
		default:
			words = append(words, "-"+letter)
		}
	}
	options, operands, err := parseAppletOptions(ctx, words, "chamf", "rdt")
	if err != nil {
		return err
	}
	if options.has('r') && options.has('t') {
		return errors.New("-r and -t cannot both be given")
	}
	if len(operands) == 0 {
		return missingOperand()
	}
	view := ProcessViewFromContext(ctx)
	now := time.Now()
	stamp := touchStamp{access: now, modified: now}
	if options.has('r') {
		native, err := resolveHostPath(view, options.value('r'))
		if err != nil {
			return err
		}
		info, err := os.Stat(native)
		if err != nil {
			return cannotStat(options.value('r'), err)
		}
		stamp = touchStamp{access: accessTime(native, info), modified: info.ModTime(), explicit: true}
	}
	if date := touchDate(options); date != "" {
		when, err := parseDateString(date, now)
		if err != nil {
			return err
		}
		stamp = touchStamp{access: when, modified: when, explicit: true}
	}
	// -a alone leaves the modification time as it is, -m alone the access time: a zero time is
	// what os.Chtimes leaves alone.
	if options.has('a') && !options.has('m') {
		stamp.modified = time.Time{}
	} else if options.has('m') && !options.has('a') {
		stamp.access = time.Time{}
	}
	touched := true
	for _, path := range operands {
		// A path the shell's view refuses, a disabled /cygdrive, is returned as it is.
		native, err := resolveHostPath(view, path)
		if err != nil {
			return err
		}
		if err := stamp.apply(native, createMode(view, 0o666), options.has('c'), options.has('h')); err != nil {
			fmt.Fprintf(stderr, "touch: %v\n", operandFailure(path, err))
			touched = false
		}
	}
	if !touched {
		return ExitStatus(1)
	}
	return nil
}

// touchDate is -d's or -t's DATE, whichever came last, as the two share busybox's one variable.
func touchDate(options appletOptions) string {
	switch options.last("dt") {
	case 'd':
		return options.value('d')
	case 't':
		return options.value('t')
	}
	return ""
}

// touchStamp is the times to set, a zero one left alone. explicit is whether they were given,
// by -r, -d or -t, and so are set on a file touch creates too.
type touchStamp struct {
	access, modified time.Time
	explicit         bool
}

// apply sets native's times, creating it with perm if it is not there unless noCreate. With
// noDereference a symbolic link's own times are set.
func (s touchStamp) apply(native string, perm os.FileMode, noCreate, noDereference bool) error {
	var err error
	if info, statErr := os.Lstat(native); noDereference && statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		err = setLinkTimes(native, s.access, s.modified)
	} else {
		err = os.Chtimes(native, s.access, s.modified)
	}
	if err == nil || !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if noCreate {
		return nil
	}
	file, err := os.OpenFile(native, os.O_CREATE|os.O_RDWR, perm)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil || !s.explicit {
		return err
	}
	return os.Chtimes(native, s.access, s.modified)
}
