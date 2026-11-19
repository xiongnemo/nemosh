package applets

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// systime, strftime and mktime are busybox's (editors/awk.c:426) as well as gawk's, and were
// `call to undefined function` here: `awk '{ print strftime("%F %T"), $0 }'` is how a log line
// is stamped. strftime is the one date and ts use, so it has every conversion glibc has, where
// busybox-w32's goes through MSVCRT and answers nothing at all for %F, %T or anything it does not
// know. The optional last argument is gawk's: true for UTC.
func TestAwkTime_stampsAndReadsDates(t *testing.T) {
	local := func(seconds int64, layout string) string { return time.Unix(seconds, 0).Format(layout) }
	for _, test := range []struct{ program, want string }{
		{`BEGIN{print strftime("%Y-%m-%d %H:%M:%S", 0, 1)}`, "1970-01-01 00:00:00"},
		{`BEGIN{print strftime("%F %T %j", 1795131000, 1)}`, "2026-11-19 23:30:00 323"},
		{`BEGIN{print strftime("%F %T", 86400)}`, local(86400, "2006-01-02 15:04:05")},
		{`BEGIN{print strftime("%H:%M", "3600")}`, local(3600, "15:04")},
		{`BEGIN{print strftime("%Y", 1e12, 1)}`, "33658"},
		// An unknown conversion is left as written, as glibc leaves it.
		{`BEGIN{print strftime("%Q|%", 0, 1)}`, "%Q|%"},
		{`BEGIN{print "<" strftime("") ">"}`, "<>"},
		{`BEGIN{print mktime("1970 01 01 00 00 00")}`, strconv.FormatInt(time.Date(1970, 1, 1, 0, 0, 0, 0, time.Local).Unix(), 10)},
		{`BEGIN{print mktime("1970 01 01 00 00 00", 1)}`, "0"},
		{`BEGIN{print mktime("2026 11 19 23 30 00", 1)}`, "1795131000"},
		// Out of range is carried into the next field, as C's mktime carries it.
		{`BEGIN{print mktime("2026 13 40 10 30 00")}`, strconv.FormatInt(time.Date(2026, 13, 40, 10, 30, 0, 0, time.Local).Unix(), 10)},
		// Fewer than six fields, a month before 1 or a year before 1900 is -1, as busybox's is.
		{`BEGIN{print mktime("2026 11 20 10 30")}`, "-1"},
		{`BEGIN{print mktime("1899 11 20 10 30 00")}`, "-1"},
		{`BEGIN{print mktime("2026 0 20 10 30 00")}`, "-1"},
		{`BEGIN{print mktime("x")}`, "-1"},
		{`BEGIN{print strftime("%F", mktime("2026 11 20 10 30 00"))}`, "2026-11-20"},
	} {
		if got, stderr, status := runAwk(t, test.program, ""); got != test.want+"\n" || status != 0 {
			t.Errorf("%s: %q (status %d, %s), want %q", test.program, got, status, stderr, test.want)
		}
	}
}

// The default format is busybox's, "%a %b %d %H:%M:%S %Z %Y", and the time is now.
func TestAwkTime_defaultsToNow(t *testing.T) {
	before := time.Now()
	stamp, stderr, status := runAwk(t, `BEGIN{print strftime(); print systime()}`, "")
	after := time.Now()
	lines := strings.Split(strings.TrimSuffix(stamp, "\n"), "\n")
	if status != 0 || len(lines) != 2 {
		t.Fatalf("strftime(); systime() = %q (status %d, %s)", stamp, status, stderr)
	}
	const layout = "Mon Jan 02 15:04:05 MST 2006"
	if lines[0] != before.Format(layout) && lines[0] != after.Format(layout) {
		t.Errorf("strftime() = %q, want %q", lines[0], before.Format(layout))
	}
	seconds, err := strconv.ParseInt(lines[1], 10, 64)
	if err != nil || seconds < before.Unix() || seconds > after.Unix() {
		t.Errorf("systime() = %q, want between %d and %d", lines[1], before.Unix(), after.Unix())
	}
}

// mktime's seventh field says whether daylight saving was in force at the time given, as C's
// tm_isdst does: 0 reads the clock as standard time, 1 as daylight time, and -1 or none lets the
// zone say. The values are busybox-w32's, measured in Sydney, where November is in daylight time
// and July is not.
func TestAwkTime_mktimeTakesTheDaylightField(t *testing.T) {
	sydney, err := time.LoadLocation("Australia/Sydney")
	if err != nil {
		t.Skipf("no zone database: %v", err)
	}
	for _, test := range []struct {
		spec string
		want float64
	}{
		{"2026 11 20 10 30 00 0", 1795134600},
		{"2026 11 20 10 30 00 1", 1795131000},
		{"2026 11 20 10 30 00 -1", 1795131000},
		{"2026 11 20 10 30 00", 1795131000},
		{"2026 07 20 10 30 00 0", 1784507400},
		{"2026 07 20 10 30 00 1", 1784503800},
	} {
		if got := awkMktime(test.spec, sydney); got != test.want {
			t.Errorf("mktime(%q) in Sydney = %.0f, want %.0f", test.spec, got, test.want)
		}
	}
}

// and, or, xor, compl, lshift and rshift are busybox's too. The operands are truncated and taken
// as unsigned, a negative one in two's complement, as busybox takes them -- at gawk's 53 bits,
// which a double holds exactly, rather than busybox's C long: 32 bits on Windows wraps
// `lshift(1, 40)` to 256, and 64 on Linux makes compl(5) a double that has lost its low bits.
// gawk's and, or and xor take more than two operands.
func TestAwkBitwise_isUnsigned53Bit(t *testing.T) {
	for _, test := range []struct{ program, want string }{
		{`BEGIN{print and(5.9, 3), or(4, 1), xor(6, 3), xor(1.5, 1)}`, "1 5 5 0"},
		{`BEGIN{print lshift(1, 40), rshift(256, 4), lshift(3, 0)}`, "1099511627776 16 3"},
		{`BEGIN{print and(compl(5), 7), and(-1, 255), rshift(-1, 49)}`, "2 255 15"},
		{`BEGIN{print compl(0), and(compl(0), 255), lshift(1, 64)}`, "9007199254740991 255 0"},
		{`BEGIN{print and(7, 6, 4), or(1, 2, 4, 8), xor(1, 3, 7)}`, "4 15 5"},
	} {
		if got, stderr, status := runAwk(t, test.program, ""); got != test.want+"\n" || status != 0 {
			t.Errorf("%s: %q (status %d, %s), want %q", test.program, got, status, stderr, test.want)
		}
	}
	for _, program := range []string{`BEGIN{print and(1)}`, `BEGIN{print compl(1, 2)}`, `BEGIN{print lshift(1)}`, `BEGIN{print mktime()}`, `BEGIN{print systime(1)}`} {
		if _, err := parseAwkProgram(program); err == nil {
			if _, stderr, status := runAwk(t, program, ""); status == 0 {
				t.Errorf("%s succeeded: %s", program, stderr)
			}
		}
	}
}
