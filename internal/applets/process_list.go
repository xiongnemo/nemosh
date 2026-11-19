package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/xiongnemo/nemosh/internal/proc"
	"github.com/xiongnemo/nemosh/internal/textgrid"
)

// ps lists processes, and reads its arguments as procps-ng's does: `ps aux`, `ps -ef`,
// `ps -eo pid,etime,args --sort=-rss`, `ps -p $$ -o comm=`, `ps axf`. See process_list_options.go
// for what it takes and process_list_fields.go for the columns.
//
// Without a format it prints the columns it always has here -- busybox-w32's
// `PID PPID USER TIME ELAPSED COMMAND` has a thread count and a resident size here where its USER
// and ELAPSED are -- because a script reads them; every process, as busybox lists them.
//
// Everything but two columns comes from the system table, which answers for every process without
// a handle on any (see internal/proc). The two are the owner and the command line, which need one,
// and Windows refuses an unelevated session the processes of another account: measured, 176 of 436
// opened. Those print as procps prints what it cannot have -- USER as `?` and the command as its
// image name in brackets -- rather than as busybox-w32's `root`, which names an owner nobody read.
// TTY is `?` for every process, there being no controlling terminal on Windows, and STAT is
// derived from the threads' states (see proc.State), since Windows keeps none for a process.
func newPsApplet() Applet {
	return simpleApplet{name: "ps", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, err := parsePsArgs(args)
		if err != nil {
			return err
		}
		snapshot, err := proc.NewSampler().Sample(false)
		if err != nil {
			// Reported, not answered with an empty list: "nothing is running" and
			// "I cannot see" are different, and only one of them is safe to act
			// on. The same rule pgrep follows.
			return err
		}
		table := psTable{taken: snapshot.Taken, memory: snapshot.Memory.TotalPhysical,
			processes: snapshot.Processes, details: proc.NewDetailCache(), self: processAccountName()}
		return writePs(ctx, stdout, options, table, options.lineWidth(psTerminalWidth(stdout)))
	}}
}

// writePs prints the table as the options ask. Nothing selected is the header alone and a status
// of 1, as procps has it, so `ps -p $pid >/dev/null` asks whether a process runs.
func writePs(ctx context.Context, stdout io.Writer, options psOptions, table psTable, width int) error {
	columns := options.layout()
	rows := options.order(options.selected(&table))
	out := bufio.NewWriter(stdout)
	if !options.noHeader && slices.ContainsFunc(columns, func(c psColumn) bool { return c.header != "" }) {
		writePsLine(out, psLine(columns, func(_ int, column psColumn) string { return column.header }), width)
	}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		writePsLine(out, psLine(columns, func(index int, column psColumn) string {
			return psCell(column, row, index == len(columns)-1)
		}), width)
	}
	if err := out.Flush(); err != nil {
		return err
	}
	if len(rows) == 0 {
		return ExitStatus(1)
	}
	return nil
}

func writePsLine(out *bufio.Writer, line string, width int) {
	out.WriteString(truncateToCells(line, width))
	out.WriteByte('\n')
}

// layout is the columns of the format asked for.
func (o psOptions) layout() []psColumn {
	var spec string
	switch o.style {
	case psCustom:
		return o.columns
	case psFull:
		spec = "user=UID,pid,ppid,c,stime,tname,time,cmd"
	case psUserStyle:
		spec = "user,pid,%cpu,%mem,vsz,rss,tname,stat,start_time,bsdtime,args"
	default:
		fields := psDefaultFields()
		columns := make([]psColumn, len(fields))
		for index, field := range fields {
			columns[index] = psColumn{field: field, header: field.header, width: field.width}
		}
		return columns
	}
	columns, err := parsePsFormat(spec)
	if err != nil {
		panic(fmt.Sprintf("ps: the %q format: %v", spec, err))
	}
	return columns
}

// psCell is one process's text in one column: a name cut with a `+` where it is too long for a
// column that is not the last, as procps marks it, and a command after its place in the tree.
func psCell(column psColumn, row *psRow, last bool) string {
	text := column.field.cell(row)
	if column.field.name && !last && textgrid.Cells(text) > column.width {
		return truncateToCells(text, column.width-1) + "+"
	}
	if column.field.command {
		return row.prefix + text
	}
	return text
}

// psLine lays the cells out as procps does. Each column has a place to end; a value too wide
// for its column pushes what follows along, and the columns after it take the room back out of
// their padding, keeping a blank between, so one long number does not shift the rest of the line.
func psLine(columns []psColumn, cell func(int, psColumn) string) string {
	var line strings.Builder
	end, at := 0, 0
	for index, column := range columns {
		text := cell(index, column)
		cells := textgrid.Cells(text)
		if index > 0 {
			end++
		}
		end += column.width
		pad := end - column.width - at
		if column.field.right {
			pad = end - at - cells
		}
		if index > 0 {
			pad = max(pad, 1)
		}
		pad = max(pad, 0)
		line.WriteString(strings.Repeat(" ", pad))
		line.WriteString(text)
		at += pad + cells
	}
	return line.String()
}

// lineWidth is where a line is cut: at the terminal's width, as procps cuts it, at 132 or more for
// one w, nowhere for two or for output that is not a terminal, and at --width's if it was given.
func (o psOptions) lineWidth(terminal int) int {
	switch {
	case o.width > 0:
		return o.width
	case terminal <= 0 || o.wide >= 2:
		return 0
	case o.wide == 1:
		return max(terminal, 132)
	}
	return terminal
}

func psTerminalWidth(stdout io.Writer) int {
	if file := stdoutFile(stdout); file != nil {
		if width, _, err := term.GetSize(int(file.Fd())); err == nil && width > 0 {
			return width
		}
	}
	return 0
}

// processCPUTime is cumulative CPU as `H:MM:SS`, the TIME of ps without a format.
//
// Hours rather than days, because a process that has used more than a day of CPU is better shown
// as three digits of hours than in a second unit the column would have to widen for.
func processCPUTime(used time.Duration) string {
	hours := int(used / time.Hour)
	minutes := int(used/time.Minute) % 60
	seconds := int(used/time.Second) % 60
	return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
}
