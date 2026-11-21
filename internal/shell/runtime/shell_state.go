package runtime

import (
	"sync"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// shellState is what every copy of a Runtime made between two snapshots shares: the tables a
// command reads and writes through, set when the shell or a snapshot of it is made (clone,
// newRuntimeWithState). A copy that is to have one of them to itself -- a command's temporary
// assignments their own variables, a background job its own traps -- detaches first, which is
// what copying the Runtime did for these when they were on it.
type shellState struct {
	initErr   error
	registry  applets.Registry
	functions map[functionName]functionDefinition
	vars      map[string]string
	traps     map[trapName]string
	// listedTraps are the shell's traps a subshell does not inherit, DEBUG and ERR, which it
	// lists all the same until it sets or resets them; see trap_listed.go.
	listedTraps map[trapName]string
	trapRunning map[trapName]bool
	// signals holds what `kill` sent this shell until a command boundary runs the trap;
	// see signal_inbox.go. Nil in a subshell, which a signal is never addressed to.
	signals   *signalInbox
	options   *shellOptions
	expansion *expansionState
	aliases   map[string]string
	// childCPU is shared by pointer across snapshots: a pipeline stage's
	// children are the shell's children too, and `times` in the parent has to
	// see what they used.
	childCPU *childCPUTime
	// history is shared by pointer across snapshots, so a command recorded in a
	// pipeline stage is still there when the parent's `history` asks.
	history *shellHistory
	// completions is what complete was told; see complete_builtin.go. Shared as history is.
	completions *completionTable
	// bindings is what bind set, which the line editor reads keys through; see
	// readline_bindings.go. Shared as completions is.
	bindings *keyBindings
	// commands is where the commands that ran were found; see command_hash.go. A subshell
	// starts with the shell's and keeps its own.
	commands *commandTable
	// dirStack is what lies beneath the current directory, for pushd/popd/dirs.
	// Position zero is not stored -- it is read from the shell, so it cannot go
	// stale when `cd` moves. See builtin_dirs.go.
	dirStack *directoryStack
	// arrays is the indexed-array store, kept apart from vars because packing
	// elements into one string cannot represent an element containing the
	// separator -- which is the case arrays exist for. See array.go.
	arrays *shellArrays
	// loops carries the pending `break n` / `continue n` level. Shared by pointer
	// for the reason loop_levels.go gives: only loops read it, and threading it
	// through every flowControl signature would touch everything that does not.
	loops *loopLevels
	// special holds what $RANDOM and $SECONDS are computed from. Shared by
	// pointer so a subshell keeps counting from the same start.
	special *specialState
	// substitutions counts the `>(cmd)` commands still reading, which a script waits for
	// before it exits. One for the whole shell, shared by every snapshot: a pipeline stage
	// or a subshell can start one too. See output_substitution.go.
	substitutions *sync.WaitGroup
	// appletFrames are the frames applet calls were handed recently, which a call the same
	// as one of them is handed again; see applet_frame.go. Shared by every snapshot.
	appletFrames *appletFrames
	readonly     map[string]struct{}
	attributes   map[string]variableAttributes
	mutatedVars  map[string]struct{}
	mask         *fileModeMask
	paths        *pathState
	env          Environment
	jobScope     *jobScope
	lifecycle    *shellLifecycle
}

// detach gives r a shellState of its own, a copy of the one it shared, so that replacing one of
// its tables replaces it for r alone.
func (r *Runtime) detach() {
	state := *r.shellState
	r.shellState = &state
}
