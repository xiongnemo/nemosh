package main

import (
	"context"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// The default prompt carries colour, because a prompt that does not is the
// first thing a user replaces. Only the eight original foreground colours and
// bold are used: those survive every terminal Nemosh runs on, including a plain
// conhost, where the 256-colour and truecolour forms do not.
//
// Every sequence is closed with a reset, so a prompt cannot leave the terminal
// tinted for the command that follows.
const (
	promptReset  = "\033[0m"
	promptBlue   = "\033[1;34m"
	promptGreen  = "\033[1;32m"
	promptYellow = "\033[0;33m"
	promptRed    = "\033[1;31m"
	promptDim    = "\033[2m"
)

var (
	defaultPS1 = promptBlue + `# \u` + promptReset + promptDim + ` @ ` + promptReset +
		promptGreen + `\h` + promptReset + promptDim + ` in ` + promptReset +
		promptYellow + `\w` + promptReset + "\n" + promptRed + `\$` + promptReset + ` `
	defaultPS2 = promptDim + `>` + promptReset + ` `
)

func interactivePrompt(rt runtime.Runtime, continuation bool) string {
	return interactivePromptWithStatus(context.Background(), rt, continuation, 0)
}

// The prompt is expanded before its backslash escapes are rendered, busybox's order; see
// runtime.Prompt. Rendering first would feed a directory name back into the parser, so a
// directory called `$(...)` would run it.
func interactivePromptWithStatus(ctx context.Context, rt runtime.Runtime, continuation bool, lastStatus int) string {
	name, fallback := "PS1", defaultPS1
	if continuation {
		name, fallback = "PS2", defaultPS2
	}
	value, present := rt.LookupVariable(name)
	if !present {
		value = fallback
	}
	return rt.Prompt(ctx, value, lastStatus)
}
