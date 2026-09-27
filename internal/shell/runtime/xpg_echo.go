package runtime

// xpgEchoArgs is `shopt -s xpg_echo` for the echo applet: backslash escapes are expanded
// unless -E says not, as bash's echo expands them. busybox has no shopt, and its echo -- this
// one -- expands them only when -e asks, so an -e goes in front, and a later -E still wins.
// Only the applet: a program named echo on PATH is what it is.
func (r Runtime) xpgEchoArgs(args []string) []string {
	if !r.options.xpgEcho || args[0] != "echo" {
		return args
	}
	return append([]string{"echo", "-e"}, args[1:]...)
}
