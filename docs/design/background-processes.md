# Background jobs as processes

Status: **steps 1-3 built** behind `NEMOSH_JOBS=process` (see "How it lands"), 2026-09-24.
The goroutine is still the default. This is the design the bash-compatibility plan put
last, and asked for before any code.

## What is wrong with the goroutine

A background job, `cmd &`, is a goroutine running over a snapshot of the shell
(`snapshot.go`, `execute_ast.go:launchBackground`). That is cheap -- microseconds -- and
it is right about almost everything: the job sees the shell's variables and functions,
cannot change them, and runs concurrently. What it cannot be is a process, and five
things a script can observe need one:

| observable | now | with a process |
| --- | --- | --- |
| `$!` | `%1`, a job spec | the pid, as both references give |
| `$BASHPID` | unset | the job's own pid |
| `wait $pid`, `kill $pid`, `tasklist`, `taskkill` | only `%N` and the spec in `$!` work | the pid works everywhere |
| `trap '...' TERM` inside the job | could not fire: nothing delivered TERM to a goroutine. It does now, through the inbox both launchers share (step three) | the parent can tell the child |
| `kill %1` on a job that started programs | cancels the job's context, which reaches the programs it started under `exec.CommandContext`, not their children | a Job Object takes the whole tree |

The corpus records the first two as gaps (`bash_gaps.json`: the numeric `$!`, BASHPID),
and `coproc` needs a real child to name in `COPROC_PID`.

Suspension stays out: `Ctrl-Z`, `bg`, `kill -STOP/-CONT` are refused for reasons that are
about Windows, not goroutines (support-matrix.md, Process control). A process does not
change that.

## What busybox-w32 does

It has no fork either, and its background jobs are processes all the same
(`shell/ash.c`, busybox-w32 v1.38.0):

- `spawn_forkshell` (17039) deep-copies the variables, functions, aliases, history and
  job table into a `CreateFileMapping` region with an inheritable handle (17766-17849),
  and starts `busybox sh --fs <handle>`.
- The child maps the region, rebases the pointers it holds, and carries on from the node
  it was given (17852-17972).
- `forkparent` sets `$!` from `GetProcessId` (6207-6234), and `wait` is
  `WaitForMultipleObjects` over the process handles (4899).
- A background child ignores Ctrl-C and gets `/dev/null` as stdin (17935).
- `kill -9` is `TerminateProcess`; every other signal injects a remote thread that calls
  `ExitProcess(sig << 24)`, so the parent's `wait` sees 128+n (`win32/process.c:851-909`).

The shape to copy is the first four. The fifth kills without running the child's traps;
this design does better there, because the child is this shell and can be asked.

## The design

### 1. The child is `nemosh --job`

A hidden entry point in `cmd/nemosh`: `nemosh --job <handle>` reads a job description
from the inherited handle, rebuilds a Runtime from it, runs the job's program, and exits
with its status. The description is written by the parent into an anonymous pipe whose
read end is the one extra handle the child inherits; its number travels in argv, as
busybox's `--fs` does. On other platforms the pipe is `ExtraFiles[0]`, fd 3.

A pipe rather than a file mapping, because the description is read once, front to back,
and a pipe needs no size decided in advance and leaves nothing to unmap.

### 2. The program travels as printed text

busybox copies the node tree and rebases pointers. The first version of this design had
the equivalent -- `encoding/gob` over the AST types -- and it does not work as stated: gob
encodes exported fields only, and every field of every node in `ast.go` and `ast_word.go`
is unexported. Making it work would mean a mirror type per node, kept in step with the
real ones by hand: the two-copies problem again, at the scale of the whole grammar.

So the job's program travels as text, printed from its nodes by the printer `declare -f`
uses (script_print.go), and the child parses it. The objection to text was heredocs --
the parser takes their bodies out before it reads anything, so no line it holds contains
them -- and the printer answers it: it writes each body back after the line its operator
is on. `TestDeclareF_readsBackAsTheSameFunction` is the evidence: a function using every
construct the printer knows, printed, evaluated under another name, and run beside the
original, gives the same output. That test is the contract this step depends on, and a
construct the printer gets wrong is a job that runs differently -- so the round-trip test
grows with the grammar.

Functions travel the same way, as `declare -f` would print them. One cost: `$LINENO`
inside a job counts the printed lines, from the line the job started on, not the lines
as written. Recorded when this lands.

### 3. The state travels through one codec, with a guard

The state is a plain struct of exported fields, encoded as JSON -- it is read once, by
the same binary, and JSON is the one encoding a person can read when a job misbehaves.
Everything `clone()` copies (snapshot.go) goes into the description: variables, both
kinds of array and which indices are live, attributes, read-only and exported names
(including the pending ones, export_pending.go), functions, aliases, options and the
invocation letters, positional parameters and `$0`, the call stack and script file
(call_stack.go), umask, the directory stack, the working directory, traps (as text), the
`$SECONDS` origin and a fresh `$RANDOM` seed, function and source depth, and the
errexit exemption.

The danger is the one this repository names "two copies of one question": `clone()` and
the codec both decide what a child inherits, and a field added to `Runtime` that one of
them forgets is a job that silently differs from a subshell. So a test walks `Runtime`'s
fields with reflection and fails for any field not named in a table that says, for each
one, *encoded*, *rebuilt in the child* (the fd table, job scope, lifecycle), or *not
inherited* (locals, loops). A new field fails the build until someone decides.

A round-trip test encodes a runtime with every kind of state set, decodes it, and
compares the two with `reflect.DeepEqual` over the encoded fields.

### 4. Descriptors: inherited where they are handles, piped where they are not

A job's descriptors are whatever its fd table holds when it starts:

- a file, the console, or a pipe: the handle is inherited, listed explicitly in
  `SysProcAttr.AdditionalInheritedHandles` so nothing else leaks into the child, and the
  description maps each fd number to the handle value it arrives under;
- something in memory that the job writes to -- a test's `bytes.Buffer`, a command
  substitution's capture: the parent makes a pipe and runs a copy goroutine between it and
  the resource for the life of the job. This is what keeps every existing test that
  captures a background job's output in a buffer working unchanged.
- something in memory that the job reads -- a heredoc body, `/dev/clipboard`: what is
  left of it moves into a temporary file the first time a job is given it, and the shell
  reads from that file too (memory_input.go). A pipe filled from here would have read the
  text out of the shell's hands whether the job wanted it or not. With the file, the job
  and the shell share one offset, as they do in both references, whose heredoc is a file
  from the start: after `exec 4<<DOC`, `{ read a <&4; } & wait; read b <&4` gives the
  job the first line and the shell the second.

stdin is `/dev/null` for a job in a session with job control, as in busybox, and
otherwise what it was.

### 5. The job record learns about processes

`jobRecord` gains the process handle and pid. `$!` becomes the pid. `wait` waits on the
handle (`WaitForSingleObject`; `WaitForMultipleObjects` for `wait -n`, replacing the
`reflect.Select` in wait_next.go), and `wait $pid` finds the job by pid. `jobs -l` prints
the pid. The status arrives as the process's exit code, so `complete` stays as it is.

### 6. Signals: asked, then made

- `kill -9` / `KILL`: `TerminateJobObject` on the job's Job Object, created with
  `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, so the programs the job started go too. Exit
  code `9 << 24`, busybox's, so the status is 137.
- `TERM`, `INT`, `HUP`, `QUIT`, `USR1`, `USR2`: a message on a control pipe the child
  also inherits. The child's runtime delivers it as the signal -- a trap runs if one is
  set, which busybox cannot do -- and exits `128 + n` if the signal's default is to end
  it. If the child has not exited within a grace period (500 ms), the parent terminates
  the Job Object with exit code `n << 24`, as busybox would have from the start.
- `kill -0`: `GetExitCodeProcess`, as proc.Terminate already does for a pid.
- STOP, TSTP, CONT and the rest of what proc.ErrCannotSuspend refuses: still refused.

The child is started with `CREATE_NEW_PROCESS_GROUP`, so the console's Ctrl-C reaches
the foreground and not the background, as busybox arranges.

**As built** (step three), with what measuring changed:

- The job's trap runs when the command in progress finishes, not the moment the byte
  arrives. That is bash's rule: `(trap 'echo got' TERM; sleep 1; echo after) & kill $!`
  prints got, then after. A signal nothing catches ends the job at once, and its EXIT trap
  still runs, as bash's does. The status is 143 all the same.
- The child answers every signal, so there is **no grace period**. The child's reader is a
  goroutine that does nothing else. A job that ignores TERM (`trap '' TERM`) has to keep
  running, and a timer would have ended it; bash leaves such a job alone too. KILL is
  still there for a job that has stopped answering.
- A subshell that is the whole of a job, `( list ) &`, runs as the job itself, as bash
  runs it in the one process it forks. Otherwise the trap the list sets would belong to a
  subshell inside the job, and `kill $!` would not reach it. The goroutine launcher does
  the same, and the inbox the two share (signal_inbox.go) gives both launchers the same
  answers.
- HUP, INT, QUIT and TERM only: the ones `kill` has names for. USR1 and USR2 are not in
  that table, nor in busybox's.
- The Job Object has no `KILL_ON_JOB_CLOSE`: with it, the shell exiting would take every
  job with it, where both references' jobs outlive the shell.
- A job that dies by a signal it did not catch exits with `n << 24`, or re-raises the
  signal off Windows. So the parent can tell a kill from `exit 143` and name the job
  `Terminated`, not `Done(143)`.

### 7. What stays a goroutine

Subshells `( )`, pipeline stages and command substitutions. None of them has a pid a
script can see, and busybox's choice to make each a process is the cost this avoids.
The one observable difference: `$BASHPID` inside `( )` is the parent's pid here, where
bash gives the subshell's. Recorded in the support matrix when this lands.

## Cost

A process start is about 9 ms (startup-and-footprint.md), plus the encode and decode,
not yet measured, and 5-10 MB of memory per running job. A goroutine costs
microseconds. `for i in $(seq 100); do :& done; wait` is benchmarked before and after,
and the result goes in startup-and-footprint.md. A script that starts thousands of
background jobs will notice; busybox pays the same price for the same reason.

## How it lands

Behind `NEMOSH_JOBS=process` until it reaches parity, then the default, then the switch
goes:

1. The codec and its two tests (reflect guard, round trip), with nothing using them; the
   printer's round-trip test widened to every construct a job can hold.
2. `nemosh --job` and the launch: pid, handle, `$!`, `wait`, status. Every existing
   background-job test runs under both launchers.
3. Signals: the Job Object, the control pipe, `kill` by pid, TERM traps in a job.
4. `$BASHPID`, `jobs -l`, `coproc` over the same launch.
5. The default flips; manual-checks.md gains `$!` in `tasklist`, `kill -0` in a loop, and
   `trap TERM` inside a job.

## Open questions

- Whether a job started inside a function should see the function's locals as values
  (bash: yes). The codec carries current values, so yes, and nothing is restored in the
  child; to confirm against bash before step 2.
- Whether `wait` with no operands should also wait for `>(cmd)` readers, which remain
  goroutines. bash waits for the last process substitution since 5.1.
- ~~The grace period for a signal the child does not answer.~~ Settled in step three:
  there is none; see section 6.
- ~~A signal sent to the shell itself, from outside it.~~ A script now gets one
  (runtime.ReceiveSignals): a real SIGTERM, HUP or QUIT off Windows, and on Windows a
  console close, which ends the script where it is, since Windows ends the process
  within seconds. A prompt still ignores TERM, as bash's does.
