package applets

import (
	"strconv"
	"strings"
	"time"
)

// The columns ps can print, by procps-ng's names for them, with its headers and widths
// (output.c's format_array), so `ps -o pid,etime,args` reads as it does on Linux.
//
// Left out are the ones Windows has nothing behind: no nice value (a priority class is not one,
// see topPriority), no process group or session in POSIX's sense, no wait channel, no flags, no
// numeric uid but busybox's emulation of one. Asking for one is procps' unknown-specifier error
// rather than a column of guesses.

// psField is one kind of column.
type psField struct {
	header string
	width  int
	right  bool
	// name is a column of account names, cut with a `+` to its width unless it is the last.
	name bool
	// command is a column the hierarchy's prefix goes in front of.
	command bool
	cell    func(*psRow) string
	compare func(a, b *psRow) int
}

// lookupPsField is the field a format or sort specifier names. A switch rather than a table: a
// table is built before main on every start, for the one command that reads it.
func lookupPsField(name string) (psField, bool) {
	switch name {
	case "pid":
		return psCount("PID", 7, func(r *psRow) int { return r.process.PID }), true
	case "ppid":
		return psCount("PPID", 7, func(r *psRow) int { return r.process.PPID }), true
	case "user", "uname":
		return psNameField("USER"), true
	case "ruser", "euser":
		// The real and effective accounts are the one account on Windows.
		return psNameField(strings.ToUpper(name)), true
	case "group", "rgroup", "egroup":
		// Group is the user, as busybox-w32 and `id` here have it: the group 4095 is named
		// after the account.
		return psNameField(strings.ToUpper(name)), true
	case "%cpu", "pcpu":
		return psField{header: "%CPU", width: 4, right: true,
			cell:    func(r *psRow) string { return psPercent(r.cpuShare()) },
			compare: compareNumber((*psRow).cpuRatio)}, true
	case "c":
		return psField{header: "C", width: 2, right: true,
			cell:    func(r *psRow) string { return strconv.FormatInt(min(r.cpuShare()/10, 99), 10) },
			compare: compareNumber((*psRow).cpuRatio)}, true
	case "%mem", "pmem":
		return psField{header: "%MEM", width: 4, right: true,
			cell:    func(r *psRow) string { return psPercent(r.memoryShare()) },
			compare: compareNumber(func(r *psRow) uint64 { return r.process.WorkingSet })}, true
	case "vsz", "vsize":
		return psKibibytes("VSZ", 6, func(r *psRow) uint64 { return r.process.Virtual }), true
	case "rss", "rssize":
		return psKibibytes("RSS", 5, func(r *psRow) uint64 { return r.process.WorkingSet }), true
	case "rsz":
		return psKibibytes("RSZ", 5, func(r *psRow) uint64 { return r.process.WorkingSet }), true
	case "tty", "tt":
		return psTerminal("TT"), true
	case "tname":
		return psTerminal("TTY"), true
	case "stat":
		return psField{header: "STAT", width: 4, cell: (*psRow).stat, compare: compareText((*psRow).stat)}, true
	case "s", "state":
		return psField{header: "S", width: 1, cell: (*psRow).state, compare: compareText((*psRow).state)}, true
	case "start":
		return psStarted("STARTED", 8, func(r *psRow) string { return psStart(r.process.Created, r.table.taken) }), true
	case "stime":
		return psStarted("STIME", 5, (*psRow).startTime), true
	case "start_time":
		return psStarted("START", 5, (*psRow).startTime), true
	case "bsdstart":
		return psStarted("START", 6, func(r *psRow) string { return psBSDStart(r.process.Created, r.table.taken) }), true
	case "lstart":
		return psStarted("STARTED", 24, func(r *psRow) string { return psLongStart(r.process.Created) }), true
	case "etime":
		return psDuration("ELAPSED", 11, (*psRow).elapsed, psElapsed), true
	case "etimes":
		return psDuration("ELAPSED", 7, (*psRow).elapsed, psSeconds), true
	case "time", "cputime":
		return psDuration("TIME", 8, (*psRow).cpu, psCPUTime), true
	case "bsdtime":
		return psDuration("TIME", 6, (*psRow).cpu, psBSDTime), true
	case "times", "cputimes":
		return psDuration("TIME", 8, (*psRow).cpu, psSeconds), true
	case "comm", "ucomm":
		return psCommand("COMMAND", 15, (*psRow).comm), true
	case "ucmd":
		return psCommand("CMD", 15, (*psRow).comm), true
	case "args", "command":
		return psCommand("COMMAND", 27, (*psRow).args), true
	case "cmd":
		return psCommand("CMD", 27, (*psRow).args), true
	case "nlwp":
		return psCount("NLWP", 4, (*psRow).threads), true
	case "thcount":
		return psCount("THCNT", 4, (*psRow).threads), true
	}
	return psField{}, false
}

// psTerminal is a TTY column. No process has a controlling terminal on Windows, and `?` is what
// procps prints for one that has none.
func psTerminal(header string) psField {
	return psField{header: header, width: 8,
		cell: func(*psRow) string { return "?" }, compare: func(*psRow, *psRow) int { return 0 }}
}

func psCount(header string, width int, get func(*psRow) int) psField {
	return psField{header: header, width: width, right: true,
		cell:    func(r *psRow) string { return strconv.Itoa(get(r)) },
		compare: compareNumber(get)}
}

// psKibibytes is a size in KiB, as procps prints VSZ and RSS.
func psKibibytes(header string, width int, get func(*psRow) uint64) psField {
	return psField{header: header, width: width, right: true,
		cell:    func(r *psRow) string { return strconv.FormatUint(get(r)/1024, 10) },
		compare: compareNumber(get)}
}

func psNameField(header string) psField {
	return psField{header: header, width: 8, name: true, cell: (*psRow).user, compare: compareText((*psRow).user)}
}

// psStarted is a column of when the process began, which sorts by that rather than its text.
func psStarted(header string, width int, cell func(*psRow) string) psField {
	return psField{header: header, width: width, right: true, cell: cell,
		compare: compareNumber(func(r *psRow) int64 { return r.process.Created.UnixNano() })}
}

func psDuration(header string, width int, get func(*psRow) time.Duration, format func(time.Duration) string) psField {
	return psField{header: header, width: width, right: true,
		cell:    func(r *psRow) string { return format(get(r)) },
		compare: compareNumber(get)}
}

func psCommand(header string, width int, get func(*psRow) string) psField {
	return psField{header: header, width: width, command: true, cell: get, compare: compareText(get)}
}

func psSeconds(elapsed time.Duration) string {
	return strconv.FormatInt(int64(elapsed/time.Second), 10)
}

// psDefaultFields are the columns ps has printed here without a format: busybox-w32's, with the
// thread count and resident size where its USER and ELAPSED are.
func psDefaultFields() []psField {
	pid, _ := lookupPsField("pid")
	ppid, _ := lookupPsField("ppid")
	threads := psCount("THR", 5, (*psRow).threads)
	resident := psField{header: "RSS", width: 9, right: true,
		cell: func(r *psRow) string {
			return humanBlocks(int64(r.process.WorkingSet+duBlock-1) / duBlock)
		},
		compare: compareNumber(func(r *psRow) uint64 { return r.process.WorkingSet })}
	cpu := psDuration("TIME", 9, (*psRow).cpu, processCPUTime)
	return []psField{pid, ppid, threads, resident, cpu, psCommand("COMMAND", 0, (*psRow).comm)}
}
