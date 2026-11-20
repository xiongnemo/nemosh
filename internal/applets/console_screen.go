package applets

import (
	"context"
	"os"

	"github.com/gdamore/tcell/v2"
)

// newConsoleScreen is the screen a full-screen applet -- vi, nano and micro, less, top -- draws
// on. tcell reads the terminal's TERM from the process's environment when a screen starts
// (tscreen.go:323): with none, on Windows, it asks the console for win32-input-mode alone, and
// with one for xterm's modifyOtherKeys and kitty's keyboard protocol as well. The TERM that
// counts is the shell's: one the session set aside when the environment brought it is not the
// process's either, and one set at the prompt or in the rc file is. The process's is made the
// shell's first; the shell gives its children its own environment, not the process's.
func newConsoleScreen(ctx context.Context) (tcell.Screen, error) {
	useShellsTerm(ProcessViewFromContext(ctx))
	return tcell.NewScreen()
}

// useShellsTerm makes the process's TERM the one view has, or none.
func useShellsTerm(view ProcessView) {
	if term, set := view.LookupEnv("TERM"); set && term != "" {
		_ = os.Setenv("TERM", term)
		return
	}
	_ = os.Unsetenv("TERM")
}
