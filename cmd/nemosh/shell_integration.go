package main

import (
	"fmt"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// Windows Terminal reads marks a shell writes around its prompt and its commands, its shell
// integration (learn.microsoft.com/windows/terminal/tutorials/shell-integration): OSC 133;A
// where a prompt starts and B where it ends and input begins, C where the command starts to run,
// and D where it finished, with its status, which the terminal colours the scrollbar's mark by
// and lets a command and its output be jumped to and selected. OSC 9;9 says the directory, which
// is where a duplicated tab or a split pane opens. A shell has to write them for itself, and
// busybox writes none.
//
// They are written in Windows Terminal, which says it is there with WT_SESSION, when the prompt
// goes to a terminal: NEMOSH_SHELL_INTEGRATION=never keeps them out, and always writes them in
// any terminal. Every mark ends in BEL, which promptColumns counts as no column. A continuation
// prompt is not marked; a command's C is written as it starts, before PS0, and its D before the
// next prompt, bare after a line that ran nothing.

const shellIntegrationSetting = "NEMOSH_SHELL_INTEGRATION"

// shellMarksOn is whether a session writes the marks, asked at each prompt.
func shellMarksOn(lookup func(string) (string, bool), stderrIsTerminal bool) bool {
	switch setting, _ := lookup(shellIntegrationSetting); setting {
	case "never":
		return false
	case "always":
		return true
	}
	_, windowsTerminal := lookup("WT_SESSION")
	return windowsTerminal && stderrIsTerminal
}

// shellMarks is where a session is in the marks' cycle: whether a prompt was marked since the
// last D, and whether a command began to run after it.
type shellMarks struct{ prompted, ran bool }

// finished is the D that ends the last prompt's cycle: the status of the command that ran, or
// none when nothing did, as after an empty line or Ctrl-C.
func (m *shellMarks) finished(status int) string {
	mark := ""
	switch {
	case m.ran:
		mark = fmt.Sprintf("\x1b]133;D;%d\a", status)
	case m.prompted:
		mark = "\x1b]133;D\a"
	}
	*m = shellMarks{}
	return mark
}

// prompt is a primary prompt with its marks, and the directory where there is one to say.
func (m *shellMarks) prompt(prompt, directory string) string {
	m.prompted = true
	var marked strings.Builder
	marked.WriteString("\x1b]133;A\a")
	if directory != "" {
		fmt.Fprintf(&marked, "\x1b]9;9;\"%s\"\a", directory)
	}
	marked.WriteString(prompt)
	marked.WriteString("\x1b]133;B\a")
	return marked.String()
}

// executing is the C a command that starts to run is marked with, after a marked prompt.
func (m *shellMarks) executing() string {
	if m == nil || !m.prompted {
		return ""
	}
	m.ran = true
	return "\x1b]133;C\a"
}

// markPrompt is the primary prompt with its marks when they are on, after writing the mark that
// ends the last cycle.
func (c command) markPrompt(marks *shellMarks, rt runtime.Runtime, prompt string, status int) string {
	if !shellMarksOn(rt.LookupVariable, c.stderrIsTerminal) {
		*marks = shellMarks{}
		return prompt
	}
	fmt.Fprint(c.stderr, marks.finished(status))
	directory, _ := nativePath(rt, ".")
	return marks.prompt(prompt, directory)
}
