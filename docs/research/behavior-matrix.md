# Architecture-Driving Behavior Matrix

This matrix records the first pass over local reference clones. It is meant to
drive the runtime architecture, not to be a full conformance table.

## Reference Entry Points

| Reference | Useful entry points |
| --- | --- |
| dash | `src/eval.c`, `src/redir.c`, `src/jobs.c`, `src/exec.c`, `src/parser.c`, `src/trap.c`, `src/var.c` |
| bash | `execute_cmd.c`, `redir.c`, `findcmd.c`, `trap.c`, `jobs.c`, `subst.c`, `variables.c`, `tests/*.sub` |
| zsh | `Src/exec.c`, `Src/parse.c`, `Src/signals.c`, `Src/jobs.c`, `Src/params.c`, `Test/*.ztst` |
| BusyBox ash | `shell/ash.c`, `shell/ash_test/**` |
| Oils | `spec/*.test.sh`, especially specs listed below with `compare_shells` metadata |
| mvdan/sh | `syntax/parser.go`, `syntax/nodes.go`, `expand/*.go`, `interp/api.go`, `interp/runner.go`, `interp/handler.go`, `interp/builtin.go` |
| Cygwin/newlib | `winsup/cygwin/fork.cc`, `spawn.cc`, `dtable.cc`, `path.cc`, `signal.cc`, `fhandler/*`, `termios.cc` |
| MSYS2 runtime | `winsup/cygwin/msys2_path_conv.cc`, `spawn.cc`, `environ.cc`, plus Cygwin-derived runtime files |
| busybox-w32 | `README.md`, `win32/process.c`, `win32/mingw.c`, `win32/popen.c`, `win32/winansi.c`, `shell/ash.c` |

## Behavior Matrix

| Behavior | POSIX/Unix reference points | Oils/spec test seeds | Go/runtime risk | Windows POSIX risk | Next experiment |
| --- | --- | --- | --- | --- | --- |
| Subshells | dash `evaltree`, `forkshell`; bash `execute_in_subshell`; BusyBox ash `forkshell`; zsh `execpline`/`execpline2` | `spec/subshell.test.sh`, `spec/background.test.sh` | A Go goroutine copy is not a fork. Need explicit shell-state snapshots for env, cwd, traps, fd table, and options. | Native Windows cannot fork process memory/fd state. Cygwin emulates this with `fork.cc`, which is too heavy for Go. | Build tiny differential cases for variable/cwd/fd/trap isolation across `( ... )`, pipeline subshells, and command substitution. |
| Command substitution | dash `evalbackcmd`; bash substitution/execute path; zsh `execsubst`; BusyBox ash `evalbackcmd` | `spec/command-sub.test.sh`, `spec/command-sub-ksh.test.sh` | Must run nested commands with captured stdout, trim trailing newlines, preserve NUL/byte policy, and isolate side effects. | Windows child process capture must avoid text-mode CRLF corruption and handle native executable quoting. | Differential tests for trailing newline trimming, nested substitutions, here-doc inside command substitution, and side effects. |
| Pipelines and exit status | dash `evalpipe`; bash `execute_pipeline`; zsh `execpline`; BusyBox ash `evalpipe` | `spec/pipeline.test.sh`, `spec/exit-status.test.sh`, `spec/background.test.sh` | Need real process graph abstraction, pipe closure discipline, `!`, `pipefail`, background status, and future job control hooks. | Windows pipes are handles; handle inheritance leaks can deadlock. Signal/SIGPIPE behavior differs. | Write tests for pipe close timing, status of last command, `!`, `pipefail`, failed exec in pipeline, and background pipeline `$!`. |
| Redirection and fd lifetime | dash `redirect`; bash `do_redirections`; zsh `addfd`/`closemn`; BusyBox ash `redirect` | `spec/redirect.test.sh`, `spec/redirect-command.test.sh`, `spec/redirect-multi.test.sh`, `spec/redir-order.test.sh` | Must model an integer fd table, dup/move/close operations, restoration scopes, expansion order, and redirection-only commands. | Windows handles are not POSIX fds. Need a translation layer with precise inheritance and cleanup. | Implement a redirection planner as a pure data structure before executing commands. Test fd 0/1/2 plus arbitrary fd cases. |
| Special built-ins | dash/busybox builtins in eval path; bash builtins and special error handling; zsh `Test/B*.ztst` | `spec/builtin-special.test.sh`, `spec/builtin-meta-assign.test.sh`, `spec/builtin-type.test.sh` | Special built-ins affect assignment persistence, fatal errors in non-interactive shells, and lookup order. Runtime must know command class before execution. | Windows does not change special-builtin semantics directly, but errors from path/fd operations feed into fatal/nonfatal decisions. | Make a special-builtin table with POSIX section links and test `export`, `readonly`, `eval`, `.`, `set`, `unset`, `trap`, `exec`. |
| Assignment/export/readonly | dash `var.c`; bash `variables.c`; BusyBox ash `setvar`; zsh `params.c` | `spec/assign.test.sh`, `spec/assign-extended.test.sh`, `spec/builtin-vars.test.sh` | Need separate shell variables, environment export view, readonly enforcement, prefix assignment scope, function/local rules. | Windows env names are case-insensitive; POSIX names are case-sensitive. Process env encoding/path-list conversion is risky. | Decide environment key normalization on Windows; test `PATH`/`Path`, `readonly`, prefix assignment before special vs regular builtins. |
| Traps and signals | dash `trap.c`; bash `trap.c`; zsh `signals.c`; BusyBox ash `dotrap`/`trapcmd` | `spec/builtin-trap.test.sh`, `spec/builtin-trap-err.test.sh`, `spec/builtin-kill.test.sh` | Shell-level traps need a scheduler integrated with command boundaries, wait, exit, and error paths. Real signal traps are harder. | Windows has console control events and process termination, not POSIX signals. Cygwin emulates signal queues and process groups. | Separate internal shell traps (`EXIT`, maybe `ERR`) from OS signal delivery. Prototype `trap EXIT` and Ctrl-C behavior later. |
| Job control | dash `jobs.c`; bash `jobs.c`; zsh `jobs.c`; BusyBox ash job sections | `spec/background.test.sh`, zsh `Test/W02jobs.ztst`, `Test/W03jobparameters.ztst` | True job control needs process groups, foreground terminal ownership, stopped/continued states, and async notifications. | Native Windows has no POSIX process groups/tty foreground control. Cygwin builds a large emulation layer. | Keep v0 non-interactive. Reserve job abstraction fields now; make job control experimental until REPL/PTY milestone. |
| PATH and executable lookup | dash `find_command`; bash `find_user_command`, `shell_execve`; BusyBox ash `find_command`; zsh `zexecve` | `spec/command_.test.sh`, `spec/builtin-meta.test.sh`, `spec/builtin-type.test.sh`, `spec/builtin-eval-source.test.sh` | Must separate aliases, functions, special builtins, regular builtins, hashed commands, PATH search, ENOEXEC fallback, and shebang. | Need policy for PATHEXT, `.bat`/`.cmd`, shebang scripts, executable bits, drive/UNC paths, and path conversion. | Build a lookup spec with expected status/diagnostics for missing, directory, non-executable, shebang, `.exe`, `.bat`, and PATH empty components. |
| Here-documents and newline handling | dash `parseheredoc`/`readtoken1`; bash parser/history heredoc logic; zsh `setheredoc`; BusyBox ash `parseheredoc` | `spec/here-doc.test.sh`, `spec/command-sub.test.sh`, `spec/builtin-read.test.sh` | Parser must preserve delimiter quoting, tab stripping, expansion mode, source locations, and redirection target fd. | CRLF/text-mode translation can change here-doc bytes. Native Windows tools may see CRLF differently from POSIX tools. | Add byte-level golden tests for quoted/unquoted heredocs, `<<-`, no final newline, command substitution, and CRLF input files. |

## mvdan/sh Assessment

`mvdan/sh` is a strong parser/AST candidate and a useful expansion reference. It
should not be treated as a complete runtime foundation for Nemosh.

Reasons to reuse parser pieces:

- `syntax.NewParser` supports POSIX mode and exposes AST nodes for calls,
  assignments, redirects, subshells, command substitutions, pipelines, and
  heredocs.
- The parser is mature Go code with a substantial ecosystem around `shfmt`.
- `expand` contains reusable ideas for word expansion, pattern matching, and
  environment handling.

Reasons not to use its interpreter as the final runtime:

- It is explicitly pure-Go and cannot provide real fork, PIDs, or POSIX fd table
  semantics.
- The interpreter uses goroutines/subshell copies for some behaviors, which is
  useful for embedding but not enough for a shell claiming POSIX behavior.
- Significant gaps remain around arbitrary fd lifetime, job control, signal
  traps, `kill`, `fg`/`bg`/`jobs`, `PIPESTATUS`/`lastpipe`, special built-in
  edge semantics, and command hashing.

Recommended path: use `mvdan/sh/syntax` as the first serious parser candidate,
borrow expansion concepts selectively, and design Nemosh's runtime substrate
from scratch around explicit process, fd, environment, and platform interfaces.

## Windows Runtime Assessment

Windows POSIX shell semantics are the core novelty and the core risk. The local
references show three possible models:

- Cygwin/MSYS2: emulate a POSIX process world over Windows with a large runtime,
  including fork, pid tables, signal queues, path conversion, fhandler objects,
  ptys, and mount logic.
- busybox-w32: implement a compact native Win32 compromise with MinGW/Win32
  APIs, simpler process handling, forward-slash path guidance, and limited POSIX
  shell/userland behavior.
- Nemosh target: avoid shipping a Cygwin-scale runtime, but still define POSIX
  shell semantics carefully enough that scripts behave predictably across
  native Windows, Linux, and macOS.

The first design document should therefore define a `platform` substrate before
implementing shell evaluation:

```text
Process spawning | fd/handle table | pipe creation | path model | executable lookup | env model | signal/trap bridge | terminal/pty hooks
```

Without that boundary, the shell runtime will accidentally bake Unix assumptions
into Go code and become hard to port.
