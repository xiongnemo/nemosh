package applets

import (
	"fmt"
	"io"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// F1, and what a legend is for.
//
// The keys used to be a single line in the status bar, which is enough to remind someone of a
// binding they already know and no use at all for the rest of it. What was missing was not more keys
// but the *terms*: the headers are four letters each, several of them name Windows concepts with no
// POSIX counterpart, and RSS against PRIV against COMMIT is three memory numbers that are all
// different and none of them self-explanatory. A monitor that can only be read by someone who has
// read its source is missing its legend.
//
// So this is a panel rather than a line, and it explains the table rather than only the keyboard.
// Every column is listed -- not only the ones currently on screen, since the layout widens with the
// window and `-o` can name any of them -- with the one-line description that lives beside the
// column in top_columns.go, so the two cannot drift apart.

// showHelp puts the legend over the table until a key dismisses it.
func (v *topView) showHelp() {
	panel := tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetWrap(true)
	panel.SetText(topHelpPanel(v.session.model))
	panel.SetBorder(true).SetTitle(" top -- what the keys do and what the columns mean ")
	panel.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp, tcell.KeyDown, tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyHome, tcell.KeyEnd:
			// The panel is longer than a terminal, so the movement keys have to reach it.
			return event
		}
		v.hideHelp()
		return nil
	})
	v.prompting = true
	v.application.SetRoot(panel, true)
}

// hideHelp puts the table back.
func (v *topView) hideHelp() {
	v.prompting = false
	v.application.SetRoot(v.root, true)
	v.application.SetFocus(v.table)
}

// topHelpPanel is the legend.
func topHelpPanel(model topModel) string {
	var out strings.Builder
	out.WriteString("[aqua]KEYS[white]\n")
	for _, group := range topKeyGroups {
		fmt.Fprintf(&out, "  [yellow]%-16s[white] %s\n", group.keys, group.what)
	}

	out.WriteString("\n[aqua]COLUMNS[white]  (")
	out.WriteString(fmt.Sprintf("%d of these are on screen now; the window's width decides, and -o overrides)\n",
		len(model.Columns)))
	for _, column := range topColumns {
		shown := " "
		if columnShown(model, column.Key) {
			// Marked, because "which of these am I looking at" is the first question
			// somebody reading a legend has.
			shown = "*"
		}
		fmt.Fprintf(&out, "  %s [yellow]%-8s[silver]%-9s[white] %s\n",
			shown, column.Header, column.Key, column.Description)
	}

	out.WriteString("\n[aqua]THE METERS AT THE TOP[white]\n")
	for _, meter := range topMeterLegend {
		fmt.Fprintf(&out, "  [yellow]%-6s[white] %s\n", meter.label, meter.what)
	}

	out.WriteString("\n[aqua]COLOUR[white]\n")
	out.WriteString("  [gray]grey[white]    nothing measured: a rate of zero, or a figure this row has not got\n")
	out.WriteString("  [green]green[white]   under a tenth of the machine\n")
	out.WriteString("  [yellow]yellow[white]  under a third\n")
	out.WriteString("  [red]red[white]     more than a third of the whole machine\n")
	out.WriteString("  [teal]cyan[white]    a memory figure\n")
	out.WriteString("  [black:aqua]header[white]  the columns; the one being sorted on is [black:green]green[white]\n")
	out.WriteString("  [black:yellow]yellow row[white]  tagged with space, which is what a multiple action would act on\n")

	out.WriteString("\n[aqua]WHAT IS NOT HERE, AND WHY[white]\n")
	for _, absent := range topAbsent {
		fmt.Fprintf(&out, "  [yellow]%-14s[white] %s\n", absent.what, absent.why)
	}
	return out.String()
}

func columnShown(model topModel, key string) bool {
	for _, column := range model.Columns {
		if column.Key == key {
			return true
		}
	}
	return false
}

// topMeterLegend explains the header, which htop explains on its own help screen and this did not.
//
// htop draws a live mock bar there with each coloured segment labelled -- low, normal, kernel, irq,
// steal, guest, io-wait for the processor, and used, shared, compressed, buffers, cache for memory.
// That is the right idea and the wrong shape to copy: those segments exist because Linux reports CPU
// time split seven ways and memory split six, and the Windows process table does not. A bar here is
// one figure, so what needs explaining is not which colour is which but *what the number is a
// fraction of* -- and for the third one, that it is not swap.
var topMeterLegend = []struct{ label, what string }{
	{"CPU", "how much of every processor was busy since the last sample, then how many there are"},
	{"Mem", "physical memory in use, then the figures: used of total, and how much of it is cache the system can reclaim"},
	{"Cmt", "commit charge -- memory the system has *promised*, against the ceiling on promises. Not swap: Windows has no measure that means what swap means, and this can run out while physical memory still looks free"},
	{"0 1 2", "the same busy fraction per processor, one meter each. Every processor gets one, so the header grows on a machine with many"},
	{"--", "no rate yet. The first sample has nothing to compare against, so the figure is unknown rather than zero"},
}

// topKeyGroups is the keyboard, grouped by what someone is trying to do rather than alphabetically.
var topKeyGroups = []struct{ keys, what string }{
	{"up down pgup", "move the cursor; it follows the process, not the row number"},
	{"/ then text", "search as you type, leaving the list whole"},
	{"n  F3", "jump to the next match"},
	{"F4  \\", "filter: hide every row that does not match"},
	{"P M T N", "sort by CPU, memory, time, pid"},
	{"1..9", "sort by that column of the ones on screen"},
	{"I", "reverse the order"},
	{"F5  t", "arrange by parentage instead of sorting flat"},
	{"+ - =", "fold or unfold a branch in that arrangement"},
	{"H", "one row per thread"},
	{"K", "show or hide Idle and System"},
	{"space", "tag a row; U clears every tag"},
	{"p", "the full path instead of the bare name"},
	{"Z", "pause: stop the list moving while you read it"},
	{"r", "sample now rather than waiting for the tick"},
	{"F7  F8", "step the priority down or up, on a process you own"},
	{"F9  k", "terminate, after a confirmation"},
	{"F1  h  ?", "this panel"},
	{"q  Esc", "quit"},
}

// topAbsent is what a person coming from htop will look for and not find, with the reason. Being
// asked "where is the load average" twice is what put this here rather than in a document.
var topAbsent = []struct{ what, why string }{
	{"load average", "Windows has no runnable-thread average. The processor queue length counts something else, so nothing is shown rather than a number under a familiar name"},
	{"TTY", "there is no controlling terminal on this platform"},
	{"nice value", "priority is a class here, not a number from -20 to 19. PRI shows the class"},
	{"disk-only IO", "the IO counters cover every handle -- file, pipe, socket. Per-process disk figures need ETW and an administrator, which this deliberately does not ask for"},
	{"a signal to kill", "Windows has no gentle one. F9 terminates, and the confirmation says so"},
	{"CPU temperature", "needs WMI or a driver, and neither belongs in an unprivileged monitor"},
}

// writeTopGlossary prints the columns and what they mean, with no terminal involved.
//
// `top -o help` and `top -s help`, after htop's `--sort-key help`. The same descriptions the F1
// panel shows, which is the point: one set of words, so a glossary that can be grepped cannot
// disagree with the one on screen. htop's own man page has drifted from its source this way -- it
// still documents IO column headers its code renamed several versions ago.
func writeTopGlossary(stdout io.Writer) error {
	if _, err := fmt.Fprintf(stdout, "%-10s %-8s %s\n", "NAME", "HEADER", "MEANING"); err != nil {
		return err
	}
	for _, column := range topColumns {
		if _, err := fmt.Fprintf(stdout, "%-10s %-8s %s\n",
			column.Key, column.Header, column.Description); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(stdout, "\nName these with -o, comma separated, or sort by one with -s.\n"+
		"The drawn form shows more of them in a wider window; F1 explains the meters and the colours.\n")
	return err
}
