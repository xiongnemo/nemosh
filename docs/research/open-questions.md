# Open Questions

This document now tracks only decisions that remain open after the current v0
scope pass. Confirmed decisions have moved into:

- `../design/v0-scope.md`
- `../design/pre-implementation-plan.md`
- `../design/windows-path-model.md`
- `../design/windows-execution-model.md`

## P0: Before Production Go Runtime Code

| Question | Why it matters | Current recommendation |
| --- | --- | --- |
| What is the exact behavior corpus file format? | Runtime implementation needs tests before behavior starts drifting. | Use TOML or YAML case files with script, expected stdout/stderr/status, platform requirements, reference shells, and known differences. |
| What is the exact Go package layout? | Package boundaries will shape parser/runtime/platform coupling. | Start from the candidate layout in `pre-implementation-plan.md` and keep Windows substrate behind interfaces. |
| What is parser slice 1? | Nemosh will use its own parser, so the first grammar cut must be small and testable. | Tokens, words, quoting, comments, CRLF handling, simple commands, assignments, and redirects. |
| What is runtime slice 1? | Runtime work must start with state and fd abstractions, not ad hoc command execution. | Shell state, variables, exported environment, options, path model, stdio fd table, and basic diagnostics. |
| What is Applet Milestone A? | The bundle direction needs an explicit first utility cut. | `true`, `false`, `echo`, `printf`, `pwd`, `env`, `printenv`, `[` / `test`. |

## P1: Before Public Compatibility Claims

| Question | Why it matters | Current recommendation |
| --- | --- | --- |
| Which differential references are required in CI? | Claims need reproducible evidence across platforms. | Native Windows busybox-w32 first, then Linux/macOS dash/bash/BusyBox where available. |
| How should Windows ACL applets be named and scoped? | The user chose native ACL applets, but exact UX is not designed. | Keep POSIX-facing `chmod` conservative; design explicit ACL utilities separately. |
| How should virtual network mounts be exposed? | The user prefers explicit applet-based virtual namespace mounting for shares. | Post-v0 or optional extension with applets such as `shares` and `nmount`; do not change `cd //host` in v0. |
| What debug flag syntax is final? | Diagnostics need consistent controls for path/exec/fd debugging. | Start with `NEMOSH_DEBUG=path,exec,fd`; refine after first runtime prototype. |

## P2: Later Milestones

| Question | Why it matters | Current recommendation |
| --- | --- | --- |
| What is the REPL architecture? | REPL requires incremental parsing, prompts, history, interrupts, raw terminal mode, completion, and multiline input. | Reserve interfaces now; implement after non-interactive script semantics are testable. |
| What is the native Windows TTY/PTY target? | `/dev/tty`, ConPTY, and full job control are hard and platform-specific. | Defer to a dedicated interactive/PTY design milestone. |
| Which zsh-like features enter first? | zsh-level capability is broad and can derail POSIX/ash core work. | First zsh-like milestone should be parser/expansion compatibility, not completion/plugins. |

## Research Tasks Still Useful

- Run small behavior probes against local busybox-w32 ash and record outputs for
  Windows-specific tests.
- Turn the confirmed path/execution decisions into behavior corpus cases.
- Inspect busybox-w32 ACL/chmod/stat behavior before designing Windows-native ACL
  applets.
- Inspect busybox-w32 installer/shim behavior further when preparing the Scoop
  manifest.
