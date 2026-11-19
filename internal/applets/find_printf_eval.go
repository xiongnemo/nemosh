package applets

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

// findPrintf is -printf's action: each part filled in for the entry, and written in one write.
// What stat would say -- the size, the mode, the owner, the identity, the times -- is what stat
// says here, asked once for the entry and only when a directive wants it, so %u is the owner ls -l
// prints and %M the mode it prints. The times are the record's too, the change time being the
// creation time, as Windows keeps no change time.
type findPrintf struct {
	parts []findFormatPart
	umask uint32
}

func (n findPrintf) eval(c findCandidate, run *findRun) bool {
	if run.err != nil {
		return true
	}
	var out strings.Builder
	facts := findFacts{candidate: c, umask: n.umask}
	for _, part := range n.parts {
		if part.stop {
			break
		}
		if part.verb == 0 {
			out.WriteString(part.text)
			continue
		}
		fmt.Fprintf(&out, part.spec, facts.directive(part))
	}
	if _, err := io.WriteString(run.stdout, out.String()); err != nil {
		run.err = err
	}
	return true
}

// findFacts answers the directives for one entry.
type findFacts struct {
	candidate       findCandidate
	umask           uint32
	record          statRecord
	asked, recorded bool
}

func (f *findFacts) stat() (statRecord, bool) {
	if !f.asked {
		f.asked = true
		f.record, f.recorded = f.candidate.statRecord(f.umask)
	}
	return f.record, f.recorded
}

func (f *findFacts) directive(part findFormatPart) string {
	c := f.candidate
	switch part.verb {
	case 'p':
		return c.display
	case 'f':
		return path.Base(c.display)
	case 'h':
		// GNU's: what is before the last slash, and . where there is none.
		if at := strings.LastIndexByte(c.display, '/'); at >= 0 {
			return c.display[:at]
		}
		return "."
	case 'P':
		return strings.TrimPrefix(strings.TrimPrefix(c.display, c.start), "/")
	case 'H':
		return c.start
	case 'd':
		return strconv.Itoa(c.depth)
	case 'y':
		if c.entry == nil {
			return "U"
		}
		return findTypeLetter(c.entry.Type())
	case 'Y':
		info, err := os.Stat(c.host)
		if err != nil {
			return "N"
		}
		return findTypeLetter(info.Mode())
	case 't', 'a', 'c':
		when := f.time(part.verb)
		return when.Format("Mon Jan _2 15:04:05") + findFraction(when) + when.Format(" 2006")
	case 'T', 'A', 'C':
		return findTimeDirective(f.time(part.verb|0x20), part.when)
	}
	record, ok := f.stat()
	if !ok {
		return ""
	}
	switch part.verb {
	case 's':
		return strconv.FormatInt(record.size, 10)
	case 'k':
		return strconv.FormatInt((record.blocks+1)/2, 10)
	case 'b':
		return strconv.FormatInt(record.blocks, 10)
	case 'm':
		octal := strconv.FormatUint(uint64(record.mode&0o7777), 8)
		if part.alternate && octal != "0" {
			octal = "0" + octal
		}
		return octal
	case 'M':
		return statModeString(record.mode)
	case 'u', 'g':
		if record.account != "" {
			return record.account
		}
		return strconv.FormatUint(uint64(record.uid), 10)
	case 'U', 'G':
		return strconv.FormatUint(uint64(record.uid), 10)
	case 'n':
		return strconv.FormatUint(record.links, 10)
	case 'i':
		return strconv.FormatUint(record.inode, 10)
	case 'D':
		return strconv.FormatUint(record.device, 10)
	case 'l':
		if record.mode&statTypeMask == statLink {
			return record.target
		}
	}
	return ""
}

// time is the entry's modification, access or change time, by %t's, %a's or %c's letter.
func (f *findFacts) time(letter byte) time.Time {
	record, ok := f.stat()
	switch {
	case !ok:
		if info, err := f.candidate.info(); err == nil {
			return info.ModTime()
		}
		return time.Time{}
	case letter == 'a':
		return record.access.Local()
	case letter == 'c':
		return record.change.Local()
	}
	return record.modify.Local()
}

// findTimeDirective is %Tk: @ the seconds since the epoch, S, T, X and + with GNU's ten digits of
// fraction after the seconds, and anything else as strftime has it.
func findTimeDirective(when time.Time, k byte) string {
	switch k {
	case '@':
		return strconv.FormatInt(when.Unix(), 10) + findFraction(when)
	case 'S':
		return when.Format("05") + findFraction(when)
	case 'T', 'X':
		return when.Format("15:04:05") + findFraction(when)
	case '+':
		return when.Format("2006-01-02+15:04:05") + findFraction(when)
	}
	return strftimeLike(when, "%"+string(k))
}

// findFraction is GNU's: the nanoseconds, nine digits, and a 0.
func findFraction(when time.Time) string {
	return fmt.Sprintf(".%09d0", when.Nanosecond())
}

// findTypeLetter is %y's letter, as -type reads the type.
func findTypeLetter(mode fs.FileMode) string {
	switch {
	case mode.IsDir():
		return "d"
	case mode&fs.ModeSymlink != 0:
		return "l"
	case mode&fs.ModeCharDevice != 0:
		return "c"
	case mode&fs.ModeDevice != 0:
		return "b"
	case mode&fs.ModeNamedPipe != 0:
		return "p"
	case mode&fs.ModeSocket != 0:
		return "s"
	}
	return "f"
}
