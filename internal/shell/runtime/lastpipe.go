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
		return stage, false, err
	}
	table, err := r.fds.clone()
	if err != nil {
		return Runtime{}, false, err
	}
	return r.withFDTable(table), true, nil
}
