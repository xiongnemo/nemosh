package runtime

import "context"

// `shopt -s lastpipe` runs a pipeline's last stage in the shell rather than in a subshell, so
// `echo y | read w` leaves w set, and so does `... | while read line; do n=...; done` leave n.
// bash does this when job control is off, which here it always is. busybox has no shopt and
// runs every stage in a subshell, as this does with the option off.
//
// The stage gets a descriptor table of its own, with the pipe for its standard input, and
// shares everything else with the shell: what it assigns is the shell's, and so are its exit,
// return and break, and a shell error it meets. In bash too `echo | exit 3` ends the script.

// stageRuntime is what a pipeline stage runs in, and whether that is the shell itself.
func (r Runtime) stageRuntime(ctx context.Context, last bool) (Runtime, bool, error) {
	if !last || !r.options.lastPipe {
		stage, err := r.snapshot(ctx)
		stage.interactive.stage = true
		if err == nil {
			stage.seeNoLoopsAsBash()
		}
		return stage, false, err
	}
	table, err := r.fds.clone()
	if err != nil {
		return Runtime{}, false, err
	}
	return r.withFDTable(table), true, nil
}

// seeNoLoopsAsBash leaves a subshell, `( ... )`, or a pipeline's stage none of the shell's loops in
// bash mode, as bash's sees none and a function's does not: break and continue there are bash's
// note that they mean nothing, and it goes on. A command substitution sees them, its break ending
// it, as in busybox, whose subshell and stage see them too, as nemosh and sh do. Measured against
// Git for Windows' bash 5.3. The subshell's loop count is its own, made by the snapshot.
func (r Runtime) seeNoLoopsAsBash() {
	if r.BashMode() {
		r.loops.depth = 0
	}
}
