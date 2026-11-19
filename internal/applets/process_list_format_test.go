package applets

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// ps as procps-ng prints it, over a table made up here: `ps aux`, `ps -ef`, `-o`, the selections
// and the sorts. The details come from a map, never from the machine -- a pid made up here is some
// real process's, see noDetails.

var psTaken = time.Date(2026, time.November, 20, 15, 0, 0, 0, time.Local)

// psDetails is a detailSource that knows what it is told.
type psDetails map[int]proc.Details

func (d psDetails) Lookup(process proc.Process) proc.Details { return d[process.PID] }

func psTestTable() psTable {
	const kib = 1024
	return psTable{
		taken:  psTaken,
		memory: 16 << 30,
		self:   "nemo",
		processes: []proc.Process{
			{PID: 0, Name: "System Idle Process", Created: psTaken.Add(-72 * time.Hour)},
			{PID: 2000, PPID: 1000, Name: "nemosh.exe", Created: psTaken.Add(-90 * time.Minute),
				User: 3 * time.Second, Kernel: time.Second, Threads: 12, Priority: 8, State: proc.StateSleeping,
				WorkingSet: 20480 * kib, Virtual: 102400 * kib},
			{PID: 4, Name: "System", Created: psTaken.Add(-72 * time.Hour),
				Kernel: 7200 * time.Second, Threads: 250, Priority: 8, State: proc.StateSleeping,
				WorkingSet: 148 * kib, Virtual: 4000 * kib},
			{PID: 1000, PPID: 900, Name: "explorer.exe", Created: psTaken.Add(-50 * time.Hour),
				User: 1000 * time.Second, Kernel: 200 * time.Second, Threads: 80, Priority: 8, State: proc.StateRunning,
				WorkingSet: 204800 * kib, Virtual: 4194304 * kib},
			{PID: 2004, PPID: 2000, Name: "worker.exe", Created: psTaken.Add(-10 * time.Minute),
				User: 30 * time.Second, Threads: 1, Priority: 6, State: proc.StateSleeping,
				WorkingSet: 1024 * kib, Virtual: 8192 * kib},
			{PID: 3000, PPID: 4, Name: "svchost.exe", Created: psTaken.Add(-72 * time.Hour),
				Threads: 5, Priority: 10, State: proc.StateSleeping,
				WorkingSet: 8192 * kib, Virtual: 16384 * kib},
		},
		details: psDetails{
			1000: {User: "nemo", CommandLine: `C:\Windows\explorer.exe`},
			2000: {User: "nemo", CommandLine: "nemosh.exe -l"},
			2004: {User: "nemo", CommandLine: "worker.exe --jobs 4"},
		},
	}
}

func runPsTable(t *testing.T, table psTable, width int, args ...string) (string, error) {
	t.Helper()
	options, err := parsePsArgs(args)
	if err != nil {
		return "", err
	}
	var stdout bytes.Buffer
	err = writePs(context.Background(), &stdout, options, table, width)
	return stdout.String(), err
}

func runPs(t *testing.T, args ...string) string {
	t.Helper()
	out, err := runPsTable(t, psTestTable(), 0, args...)
	if err != nil {
		t.Fatalf("ps %q: %v", args, err)
	}
	return out
}

func lines(rows ...string) string { return strings.Join(rows, "\n") + "\n" }

// `ps aux` is procps' u format over every process: an owner that cannot be read is `?`, a command
// line that cannot be read is the image name in brackets, and a column a value overflows gives the
// room back out of the padding of the ones after it, as procps' does.
func TestPs_auxPrintsTheUserFormat(t *testing.T) {
	row := func(cells ...any) string {
		return fmt.Sprintf("%-8s %7s %4s %4s %6s %5s %-8s %-4s %5s %6s %s", cells...)
	}
	want := lines(
		"USER         PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND",
		row("?", "4", "2.7", " 0.0", "4000", "148", "?", "Sl", "Nov17", "120:00", "[System]"),
		`nemo        1000  0.6  1.2 4194304 204800 ?      Rl   Nov18  20:00 C:\Windows\explorer.exe`,
		row("nemo", "2000", "0.0", " 0.1", "102400", "20480", "?", "Sl", "13:30", "0:04", "nemosh.exe -l"),
		row("nemo", "2004", "5.0", " 0.0", "8192", "1024", "?", "SN", "14:50", "0:30", "worker.exe --jobs 4"),
		row("?", "3000", "0.0", " 0.0", "16384", "8192", "?", "S<l", "Nov17", "0:00", "[svchost.exe]"),
	)
	for _, spelling := range [][]string{{"aux"}, {"-aux"}, {"axu"}} {
		if got := runPs(t, spelling...); got != want {
			t.Errorf("ps %s:\n%s\nwant:\n%s", spelling[0], got, want)
		}
	}
}

// `ps -ef` is the full format: the UID column names the user, as procps' does.
func TestPs_efPrintsTheFullFormat(t *testing.T) {
	row := func(cells ...any) string {
		return fmt.Sprintf("%-8s %7s %7s %2s %5s %-8s %8s %s", cells...)
	}
	want := lines(
		"UID          PID    PPID  C STIME TTY          TIME CMD",
		row("?", "4", "0", "2", "Nov17", "?", "02:00:00", "[System]"),
		row("nemo", "1000", "900", "0", "Nov18", "?", "00:20:00", `C:\Windows\explorer.exe`),
		row("nemo", "2000", "1000", "0", "13:30", "?", "00:00:04", "nemosh.exe -l"),
		row("nemo", "2004", "2000", "5", "14:50", "?", "00:00:30", "worker.exe --jobs 4"),
		row("?", "3000", "4", "0", "Nov17", "?", "00:00:00", "[svchost.exe]"),
	)
	for _, spelling := range [][]string{{"-ef"}, {"-e", "-f"}, {"-fA"}} {
		if got := runPs(t, spelling...); got != want {
			t.Errorf("ps %q:\n%s\nwant:\n%s", spelling, got, want)
		}
	}
}

// Without a format the columns are the ones ps has always printed here, whatever is selected.
func TestPs_defaultFormatIsUnchanged(t *testing.T) {
	want := lines(
		fmt.Sprintf("%7s %7s %5s %9s %9s %s", "PID", "PPID", "THR", "RSS", "TIME", "COMMAND"),
		fmt.Sprintf("%7d %7d %5d %9s %9s %s", 2000, 1000, 12,
			humanBlocks(int64(20480*1024+duBlock-1)/duBlock), processCPUTime(4*time.Second), "nemosh.exe"),
	)
	if got := runPs(t, "-p", "2000"); got != want {
		t.Errorf("ps -p 2000:\n%s\nwant:\n%s", got, want)
	}
}

func TestPs_oChoosesTheColumnsAndTheirHeaders(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-o", "pid,comm", "-p", "2000,2004"}, lines("    PID COMMAND", "   2000 nemosh.exe", "   2004 worker.exe")},
		{[]string{"-opid", "-ocomm", "-p2000"}, lines("    PID COMMAND", "   2000 nemosh.exe")},
		{[]string{"-o", "pid=", "-o", "comm=", "-p", "2000"}, lines("   2000 nemosh.exe")},
		{[]string{"-o", "pid=,comm=", "-p", "2000"}, lines("   2000 nemosh.exe")},
		// A header ends at the next comma or blank, as procps-ng's does; busybox's takes the rest
		// of the argument.
		{[]string{"-o", "pid,comm=NAME,args", "-p", "2000"}, lines(fmt.Sprintf("%7s %-15s %s", "PID", "NAME", "COMMAND"), fmt.Sprintf("%7s %-15s %s", "2000", "nemosh.exe", "nemosh.exe -l"))},
		{[]string{"-o", "comm:20,pid", "-p", "2000"}, lines(fmt.Sprintf("%-20s %7s", "COMMAND", "PID"), fmt.Sprintf("%-20s %7s", "nemosh.exe", "2000"))},
		{[]string{"-o", "pid:3=P", "-p", "2000"}, lines("  P", "2000")},
		{[]string{"-o", "user=WHO", "-p", "3000"}, lines("WHO", "?")},
		{[]string{"-o", "comm,pid", "-p", "2000"}, lines(fmt.Sprintf("%-15s %7s", "COMMAND", "PID"), fmt.Sprintf("%-15s %7s", "nemosh.exe", "2000"))},
		{[]string{"--format", "pid ppid", "--pid=2004"}, lines("    PID    PPID", "   2004    2000")},
		{[]string{"o", "pid,nlwp,thcount", "p", "4"}, lines("    PID NLWP THCNT", "      4  250   250")},
		{[]string{"-o", "pid,%cpu,pcpu,c,%mem,pmem,vsz,vsize,rss,rssize,rsz", "-p", "2004"},
			lines("    PID %CPU %CPU  C %MEM %MEM    VSZ    VSZ   RSS   RSS   RSZ", "   2004  5.0  5.0  5  0.0  0.0   8192   8192  1024  1024  1024")},
		{[]string{"-o", "pid,tty,tt,tname,stat,s,state", "-p", "2004"},
			lines("    PID TT       TT       TTY      STAT S S", "   2004 ?        ?        ?        SN   S S")},
		{[]string{"-o", "pid,user,ruser,euser,uname,group,rgroup,egroup", "-p", "2000"},
			lines("    PID USER     RUSER    EUSER    USER     GROUP    RGROUP   EGROUP", "   2000 nemo     nemo     nemo     nemo     nemo     nemo     nemo")},
		{[]string{"-o", "pid,comm,ucomm,ucmd,args,cmd,command", "-p", "4"},
			lines(fmt.Sprintf("%7s %-15s %-15s %-15s %-27s %-27s %s", "PID", "COMMAND", "COMMAND", "CMD", "COMMAND", "CMD", "COMMAND"),
				fmt.Sprintf("%7s %-15s %-15s %-15s %-27s %-27s %s", "4", "System", "System", "System", "[System]", "[System]", "[System]"))},
		{[]string{"-o", "pid,etime,etimes,time,cputime,bsdtime,times,cputimes", "-p", "4"},
			lines("    PID     ELAPSED ELAPSED     TIME     TIME   TIME     TIME     TIME", "      4  3-00:00:00  259200 02:00:00 02:00:00 120:00     7200     7200")},
		{[]string{"-o", "pid,start,stime,start_time,bsdstart,lstart", "-p", "2000"},
			lines(fmt.Sprintf("%7s %8s %5s %5s %6s %24s", "PID", "STARTED", "STIME", "START", "START", "STARTED"),
				fmt.Sprintf("%7s %8s %5s %5s %6s %24s", "2000", "13:30:00", "13:30", "13:30", "13:30", "Fri Nov 20 13:30:00 2026"))},
	} {
		if got := runPs(t, test.args...); got != test.want {
			t.Errorf("ps %q:\n%s\nwant:\n%s", test.args, got, test.want)
		}
	}
}

// A user's name longer than its column is cut to it with a `+`, procps' mark for there being more,
// unless the column is the last one.
func TestPs_aLongUserNameIsMarked(t *testing.T) {
	table := psTestTable()
	table.details = psDetails{2000: {User: "Administrator", CommandLine: "nemosh.exe"}}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-o", "user,pid", "-p", "2000"}, lines("USER         PID", "Adminis+    2000")},
		{[]string{"-o", "pid,user", "-p", "2000"}, lines("    PID USER", "   2000 Administrator")},
		{[]string{"-o", "user=THE-ACCOUNT,pid", "-p", "2000"}, lines("THE-ACCOUNT     PID", "Administra+    2000")},
	} {
		if got, err := runPsTable(t, table, 0, test.args...); err != nil || got != test.want {
			t.Errorf("ps %q: %v\n%s\nwant:\n%s", test.args, err, got, test.want)
		}
	}
}

func TestPs_selectsProcesses(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{nil, "4 1000 2000 2004 3000"},
		{[]string{"-e"}, "4 1000 2000 2004 3000"},
		{[]string{"-A"}, "4 1000 2000 2004 3000"},
		{[]string{"-p", "2000"}, "2000"},
		{[]string{"-p", "2004,1000"}, "1000 2004"},
		{[]string{"-p", "1000 2004", "-p", "4"}, "4 1000 2004"},
		{[]string{"--pid", "4"}, "4"},
		{[]string{"-q", "4"}, "4"},
		{[]string{"p", "4"}, "4"},
		{[]string{"4"}, "4"},
		{[]string{"4,3000"}, "4 3000"},
		{[]string{"--ppid", "1000"}, "2000"},
		{[]string{"--ppid=2000,4"}, "2004 3000"},
		{[]string{"-C", "nemosh"}, "2000"},
		{[]string{"-C", "NEMOSH.EXE"}, "2000"},
		{[]string{"-C", "svchost,worker"}, "2004 3000"},
		{[]string{"-u", "nemo"}, "1000 2000 2004"},
		{[]string{"-U", "NEMO"}, "1000 2000 2004"},
		{[]string{"--user=nemo"}, "1000 2000 2004"},
		{[]string{"--User", "nemo"}, "1000 2000 2004"},
		{[]string{"U", "nemo"}, "1000 2000 2004"},
		// Every selection is a union, as procps' are.
		{[]string{"-p", "4", "-C", "worker"}, "4 2004"},
		{[]string{"-N", "-u", "nemo"}, "4 3000"},
		{[]string{"--deselect", "-p", "4"}, "1000 2000 2004 3000"},
		// BSD's form without `a` is this account's processes; `x` is implied, since no process
		// has a controlling terminal for it to lift the need for.
		{[]string{"x"}, "1000 2000 2004"},
		{[]string{"u"}, "1000 2000 2004"},
		{[]string{"a"}, "4 1000 2000 2004 3000"},
		{[]string{"ax"}, "4 1000 2000 2004 3000"},
		{[]string{"axr"}, "1000"},
		{[]string{"u", "-p", "4"}, "4"},
	} {
		args := append(append([]string{}, test.args...), "-o", "pid=")
		if containsPsFormat(test.args) {
			args = test.args
		}
		got := strings.Join(strings.Fields(runPsColumn(t, args...)), " ")
		if got != test.want {
			t.Errorf("ps %q selected %q, want %q", test.args, got, test.want)
		}
	}
}

// containsPsFormat is whether a test's arguments name a format already, u, which -o would conflict with.
func containsPsFormat(args []string) bool {
	for _, arg := range args {
		if arg == "u" {
			return true
		}
	}
	return false
}

// runPsColumn is the pids ps selects, whatever its format: the second field of a u format, the
// only one of `-o pid=`.
func runPsColumn(t *testing.T, args ...string) string {
	t.Helper()
	out := runPs(t, args...)
	if !containsPsFormat(args) {
		return out
	}
	var pids []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n")[1:] {
		pids = append(pids, strings.Fields(line)[1])
	}
	return strings.Join(pids, " ")
}

// Nothing selected is the header and a status of 1, so `ps -p $pid >/dev/null` asks whether it runs.
func TestPs_nothingSelectedIsStatusOne(t *testing.T) {
	got, err := runPsTable(t, psTestTable(), 0, "-p", "99", "-o", "pid,comm")
	if status, ok := StatusCode(err); !ok || status != 1 || got != lines("    PID COMMAND") {
		t.Fatalf("ps -p 99: %q, %v; want the header and status 1", got, err)
	}
}

func TestPs_sorts(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"--sort=-rss"}, "1000 2000 3000 2004 4"},
		{[]string{"--sort", "rss"}, "4 2004 3000 2000 1000"},
		{[]string{"--sort=+rss"}, "4 2004 3000 2000 1000"},
		{[]string{"axk", "-pid"}, "3000 2004 2000 1000 4"},
		{[]string{"--sort=comm"}, "4 1000 2000 3000 2004"},
		{[]string{"--sort=-%cpu"}, "2004 4 1000 2000 3000"},
		{[]string{"--sort=user,-pid"}, "3000 4 2004 2000 1000"},
		{[]string{"--sort=start_time"}, "4 3000 1000 2000 2004"},
		{[]string{"--sort=-etime"}, "4 3000 1000 2000 2004"},
		{[]string{"--sort=-time"}, "4 1000 2004 2000 3000"},
		{[]string{"--sort=vsz"}, "4 2004 3000 2000 1000"},
		{[]string{"--sort=-nlwp"}, "4 1000 2000 3000 2004"},
	} {
		args := append(append([]string{}, test.args...), "-o", "pid=")
		if got := strings.Join(strings.Fields(runPs(t, args...)), " "); got != test.want {
			t.Errorf("ps %q ordered %q, want %q", test.args, got, test.want)
		}
	}
}

// --forest draws procps' ASCII art in front of the command, -H indents it two blanks a level, and
// siblings keep the order asked for.
func TestPs_drawsTheHierarchy(t *testing.T) {
	table := psTestTable()
	table.processes = append(table.processes, proc.Process{PID: 2008, PPID: 1000, Name: "cmd.exe",
		Created: psTaken.Add(-5 * time.Minute), Threads: 1, State: proc.StateSleeping})
	table.details.(psDetails)[2008] = proc.Details{User: "nemo", CommandLine: "cmd.exe /k"}
	forest := lines(
		"    PID COMMAND",
		"      4 [System]",
		`   3000  \_ [svchost.exe]`,
		`   1000 C:\Windows\explorer.exe`,
		`   2000  \_ nemosh.exe -l`,
		`   2004  |   \_ worker.exe --jobs 4`,
		`   2008  \_ cmd.exe /k`,
	)
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-e", "--forest", "-o", "pid,args"}, forest},
		{[]string{"axf", "-o", "pid,args"}, forest},
		{[]string{"-e", "--forest", "--sort=-pid", "-o", "pid,args"}, lines(
			"    PID COMMAND",
			`   1000 C:\Windows\explorer.exe`,
			`   2008  \_ cmd.exe /k`,
			`   2000  \_ nemosh.exe -l`,
			`   2004      \_ worker.exe --jobs 4`,
			"      4 [System]",
			`   3000  \_ [svchost.exe]`,
		)},
		{[]string{"-eH", "-o", "pid,comm"}, lines(
			"    PID COMMAND",
			"      4 System",
			"   3000   svchost.exe",
			"   1000 explorer.exe",
			"   2000   nemosh.exe",
			"   2004     worker.exe",
			"   2008   cmd.exe",
		)},
	} {
		if got, err := runPsTable(t, table, 0, test.args...); err != nil || got != test.want {
			t.Errorf("ps %q: %v\n%s\nwant:\n%s", test.args, err, got, test.want)
		}
	}
}

// On a terminal a line is cut at its width, as procps cuts it; w widens that to 132, ww or a pipe
// does not cut at all.
func TestPs_cutsLinesToTheTerminal(t *testing.T) {
	cut := lines("    PID COMMAND", `   1000 C:\Windows\e`)
	whole := lines("    PID COMMAND", `   1000 C:\Windows\explorer.exe`)
	for _, test := range []struct {
		width int
		args  []string
		want  string
	}{
		{20, nil, cut},
		{20, []string{"w"}, whole},
		{20, []string{"-w"}, whole},
		{20, []string{"-ww"}, whole},
		{0, nil, whole},
		{80, []string{"--width=20"}, cut},
		{0, []string{"--cols", "20"}, cut},
		{0, []string{"--columns=20"}, cut},
	} {
		args := append([]string{"-o", "pid,args", "-p", "1000"}, test.args...)
		options, err := parsePsArgs(args)
		if err != nil {
			t.Fatalf("ps %q: %v", args, err)
		}
		var stdout bytes.Buffer
		if err := writePs(context.Background(), &stdout, options, psTestTable(), options.lineWidth(test.width)); err != nil {
			t.Fatalf("ps %q: %v", args, err)
		}
		if got := stdout.String(); got != test.want {
			t.Errorf("ps %q on a terminal %d wide:\n%s\nwant:\n%s", args, test.width, got, test.want)
		}
	}
	if got := (psOptions{wide: 1}).lineWidth(200); got != 200 {
		t.Errorf("w on a terminal 200 wide cut at %d, want 200", got)
	}
}

func TestPs_noHeaders(t *testing.T) {
	for _, flag := range []string{"--no-headers", "--no-heading", "h"} {
		if got := runPs(t, flag, "-o", "pid", "-p", "4"); got != lines("      4") {
			t.Errorf("ps %s: %q", flag, got)
		}
	}
}

func TestPs_refusesWhatItCannotDo(t *testing.T) {
	for _, test := range []struct {
		args []string
		says string
	}{
		{[]string{"-o", "bogus"}, `unknown user-defined format specifier "bogus"`},
		{[]string{"-o", ""}, "improper format list"},
		{[]string{"-o", "pid,,comm"}, "improper format list"},
		{[]string{"-o", "comm:x"}, `invalid width in format specifier "comm:x"`},
		{[]string{"-o"}, "option requires an argument -- o"},
		{[]string{"-p", "abc"}, "process ID list syntax error"},
		{[]string{"-p", ""}, "process ID list syntax error"},
		{[]string{"--sort=bogus"}, `unknown sort specifier "bogus"`},
		{[]string{"-f", "-o", "pid"}, "conflicting format options"},
		{[]string{"aux", "-o", "pid"}, "conflicting format options"},
		{[]string{"-Z"}, "unknown option -- Z"},
		{[]string{"aZ"}, "unknown option -- Z"},
		{[]string{"--bogus"}, "unknown option -- bogus"},
		{[]string{"--width=x"}, "invalid width: x"},
		{[]string{`C:\x`}, `C:\x: neither process IDs nor BSD-style letters`},
		{[]string{"-l"}, "-l: F, PRI, NI, ADDR and WCHAN have no Windows equivalent; -f and -o FORMAT print the rest"},
		{[]string{"-elf"}, "-l: F, PRI, NI, ADDR and WCHAN have no Windows equivalent; -f and -o FORMAT print the rest"},
		{[]string{"axl"}, "-l: F, PRI, NI, ADDR and WCHAN have no Windows equivalent; -f and -o FORMAT print the rest"},
	} {
		if _, err := runPsTable(t, psTestTable(), 0, test.args...); err == nil || err.Error() != test.says {
			t.Errorf("ps %q: %v, want %q", test.args, err, test.says)
		}
	}
}

// The times as procps-ng words them.
func TestPs_timeFormats(t *testing.T) {
	day := 24 * time.Hour
	for _, test := range []struct {
		name string
		got  string
		want string
	}{
		{"etime of nothing", psElapsed(0), "00:00"},
		{"etime of a minute and a second", psElapsed(61 * time.Second), "01:01"},
		{"etime of an hour", psElapsed(time.Hour), "01:00:00"},
		{"etime of a day and more", psElapsed(day + time.Hour + time.Minute + time.Second), "1-01:01:01"},
		{"time of seconds", psCPUTime(4 * time.Second), "00:00:04"},
		{"time of a day and more", psCPUTime(day + time.Hour + time.Minute + time.Second), "1-01:01:01"},
		{"bsdtime of seconds", psBSDTime(4 * time.Second), "0:04"},
		{"bsdtime of hours", psBSDTime(2 * time.Hour), "120:00"},
		{"start today", psStart(psTaken.Add(-90*time.Minute), psTaken), "13:30:00"},
		{"start over a day ago", psStart(time.Date(2026, time.November, 8, 9, 5, 0, 0, time.Local), psTaken), "Nov 08"},
		{"stime today", psStartTime(psTaken.Add(-90*time.Minute), psTaken), "13:30"},
		{"stime yesterday within the day", psStartTime(time.Date(2026, time.November, 19, 23, 0, 0, 0, time.Local), psTaken), "Nov19"},
		{"stime last year", psStartTime(time.Date(2025, time.December, 31, 23, 0, 0, 0, time.Local), psTaken), "2025"},
		{"bsdstart today", psBSDStart(psTaken.Add(-90*time.Minute), psTaken), "13:30"},
		{"bsdstart over a day ago", psBSDStart(time.Date(2026, time.November, 8, 9, 5, 0, 0, time.Local), psTaken), "Nov  8"},
		{"lstart", psLongStart(time.Date(2026, time.November, 8, 9, 5, 7, 0, time.Local)), "Sun Nov  8 09:05:07 2026"},
		{"%cpu of a thousandth", psPercent(1), "0.1"},
		{"%cpu of more than 99.9", psPercent(1234), "123"},
	} {
		if test.got != test.want {
			t.Errorf("%s: %q, want %q", test.name, test.got, test.want)
		}
	}
}

// ps lists the process it runs in, as every ps does: `ps -p $$ -o comm=` is how a script asks
// what shell it is in, and the list pkill reads leaves itself out. Its own process is the one a
// test may ask the machine about.
func TestPs_listsItsOwnProcess(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	self := strconv.Itoa(os.Getpid())
	var stdout, stderr bytes.Buffer
	if err := newPsApplet().Run(context.Background(), []string{"-p", self, "-o", "pid=,comm="},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("ps -p %s: %v (%s)", self, err, stderr.String())
	}
	if fields := strings.Fields(stdout.String()); len(fields) != 2 || fields[0] != self ||
		!strings.EqualFold(fields[1], filepath.Base(executable)) {
		t.Fatalf("ps -p %s -o pid=,comm= printed %q, want %s %s", self, stdout.String(), self, filepath.Base(executable))
	}
}

// A control character in a command line is a `?`, so a line of ps is one line.
func TestPs_controlCharactersAreQuestionMarks(t *testing.T) {
	table := psTestTable()
	table.details = psDetails{2000: {User: "nemo", CommandLine: "nemosh.exe -c \"echo a\nb\""}}
	if got, err := runPsTable(t, table, 0, "-o", "pid=,args=", "-p", "2000"); err != nil || got != lines(`   2000 nemosh.exe -c "echo a?b"`) {
		t.Errorf("ps: %q, %v", got, err)
	}
}
