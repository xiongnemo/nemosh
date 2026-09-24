# Background jobs as processes

Status: **proposed**, 2026-09-24. Nothing here is built yet. This is the design the
bash-compatibility plan put last, and asked for before any code.

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
| `trap '...' TERM` inside the job | cannot fire: nothing delivers TERM to a goroutine | the parent can tell the child |
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

### 2. The program travels as its parsed form, not as text

busybox copies the node tree and rebases pointers. The equivalent here is to encode the
`programNode` the job runs, with `encoding/gob` over the AST types (`ast.go`,
`ast_word.go`), each node type registered once. Parent and child are always the same
binary -- the child is started from `os.Executable()` -- so the encoding needs no
compatibility across versions, and nothing needs to turn an AST back into source.

Text was the other candidate, and it loses: the parser takes heredoc bodies out before it
reads anything (`heredoc_parse.go`), so the text of `cat <<EOF` inside a job is not in any
line the parser holds, and reconstructing it is a printer this shell does not have.
Functions travel the same way: a function is a `functionDefinition` whose body is a node.

### 3. The state travels through one codec, with a guard

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
- something in memory -- a heredoc body, `/dev/clipboard`, a test's `bytes.Buffer`, a
  command substitution's capture: the parent makes a pipe and runs a copy goroutine
  between it and the resource for the life of the job. This is what keeps every existing
  test that captures a background job's output in a buffer working unchanged.

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

1. The codec and its two tests (reflect guard, round trip), with nothing using them.
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
- The grace period for a signal the child does not answer: 500 ms is a guess to be
  measured against a child busy in a loop with no trap.
