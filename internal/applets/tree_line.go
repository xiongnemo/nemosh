package applets

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// One line of tree's: the lines of the levels above it, the fields in brackets, the name, the link
// it is, and with -F what kind of thing it is, as unix.c's printinfo and printfile write them.

// write writes the line for an entry and answers whether it is listed as a directory.
func (r *treeRun) write(level int, host, display, name string, info os.FileInfo) bool {
	var line strings.Builder
	if !r.options.noIndent {
		// indent(): the levels above, then this one's ├── or └──, each with a space after it.
		for at := 1; at <= level; at++ {
			switch {
			case at < level && r.levels[at] == 1:
				line.WriteString(r.options.lines.vertical)
			case at < level:
				line.WriteString("   ")
			case r.levels[at] == 1:
				line.WriteString(r.options.lines.branch)
			default:
				line.WriteString(r.options.lines.corner)
			}
			line.WriteByte(' ')
		}
	}
	if fields := r.fields(host, info); fields != "" {
		line.WriteString("[" + fields[1:] + "]  ")
	}
	line.WriteString(name)
	isDir := info.IsDir()
	if isSymbolicLink(info) {
		if target, err := os.Readlink(host); err == nil {
			line.WriteString(" -> " + strings.ReplaceAll(target, `\`, "/"))
		}
		isDir = isDirectory(host)
		if target, err := os.Stat(host); err == nil && r.options.classify {
			line.WriteString(treeKind(host, target))
		}
	} else if r.options.classify {
		line.WriteString(treeKind(host, info))
	}
	line.WriteByte('\n')
	r.out.WriteString(line.String())
	return isDir
}

// report is unix_report's, after a blank line: the directories, and but under -d the files.
func (r *treeRun) report() {
	if r.options.noReport {
		return
	}
	plural := func(count int, one, many string) string {
		if count == 1 {
			return one
		}
		return many
	}
	fmt.Fprintf(r.out, "\n%d %s", r.dirs, plural(r.dirs, "directory", "directories"))
	if !r.options.dirsOnly {
		fmt.Fprintf(r.out, ", %d %s", r.files, plural(r.files, "file", "files"))
	}
	r.out.WriteByte('\n')
}

// fields are -p's, -u's, -g's, -s's or -h's and -D's, each after a space, as fillinfo builds them.
func (r *treeRun) fields(host string, info os.FileInfo) string {
	var fields strings.Builder
	record, recorded := statRecord{}, false
	if r.options.perms || r.options.owner || r.options.group {
		if found, err := hostStatRecord(host, false, r.umask); err == nil {
			record, recorded = found, true
		}
	}
	if r.options.perms && recorded {
		fields.WriteString(" " + statModeString(record.mode))
	}
	for _, wanted := range []bool{r.options.owner, r.options.group} {
		if wanted && recorded {
			fmt.Fprintf(&fields, " %-8s", record.account)
		}
	}
	if r.options.size {
		fields.WriteString(treeSize(info.Size(), r.options.human))
	}
	if r.options.date {
		fields.WriteString(" " + treeDate(info.ModTime(), time.Now()))
	}
	return fields.String()
}

// treeSize is psize's: the bytes in eleven columns, or under -h in four, a unit after them, as
// ls -h counts in 1024s.
func treeSize(size int64, human bool) string {
	if !human {
		return fmt.Sprintf(" %11d", size)
	}
	unit := 0
	if size >= 1024 {
		unit = 1
	}
	for ; size >= 1024*1024; unit++ {
		size /= 1024
	}
	if unit == 0 {
		return fmt.Sprintf(" %4d", size)
	}
	format := " %3.1f%c"
	if size/1024 >= 10 {
		format = " %3.0f%c"
	}
	return fmt.Sprintf(format, float64(size)/1024, "BKMGTPEZY"[unit])
}

// treeDate is do_date's: the month, the day and the time within six months, the year beyond.
func treeDate(when, now time.Time) string {
	if when.After(now) || when.Add(6*31*24*time.Hour).Before(now) {
		return strftimeLike(when, "%b %e  %Y")
	}
	return strftimeLike(when, "%b %e %R")
}

// treeKind is -F's mark: / for a directory, * for what can be run, | for a pipe, = for a socket.
func treeKind(host string, info os.FileInfo) string {
	switch mode := info.Mode(); {
	case mode.IsDir():
		return "/"
	case mode&os.ModeNamedPipe != 0:
		return "|"
	case mode&os.ModeSocket != 0:
		return "="
	case mode.IsRegular() && isExecutableFile(host, info):
		return "*"
	}
	return ""
}
