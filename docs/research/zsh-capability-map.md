# Zsh-Level Capability Map

The long-term ambition is zsh-level capability, but research should decompose
that ambition into separable domains. This prevents the project from confusing a
POSIX shell foundation with a full interactive shell ecosystem.

## Capability Domains

- POSIX sh compatibility: baseline language, built-ins, non-interactive scripts,
  redirection, functions, traps, and exit semantics.
- Script compatibility beyond POSIX: common bash/ksh/zsh extensions, arrays,
  advanced parameter expansion, extended globbing, process substitution, and
  arithmetic behavior.
- Interactive shell loop: prompt lifecycle, input editing, incremental parsing,
  multiline continuation, and interrupt behavior.
- Line editing: emacs/vi keymaps, undo, kill ring, completion menu integration,
  and terminal width handling.
- History: persistent history files, search, timestamps, deduplication, shared
  history, history expansion, and privacy controls.
- Completion: command completion, path completion, option completion, custom
  completion functions, caching, and descriptions.
- Globbing and pattern features: recursive glob, qualifiers, case behavior,
  locale interaction, and unmatched pattern policy.
- Prompt and expansion: prompt escapes, right prompt, async prompt updates,
  command substitution in prompt, and status display.
- Job control: background jobs, foreground control, stopped jobs, notifications,
  and terminal process group behavior.
- Modules/plugins: loadable or built-in modules, function autoloading, plugin
  discovery, startup files, and compatibility with existing ecosystem patterns.
- Terminal/PTY behavior: raw mode, resize handling, alternate screen avoidance,
  bracketed paste, color, and Unicode display width.
- Startup/configuration: login vs interactive vs script startup files, restricted
  shell behavior, and environment import/export.

## Research Questions Per Domain

For each domain, collect answers to:

- What does POSIX require, if anything?
- What does zsh provide that users expect?
- What does bash provide that real-world scripts rely on?
- What can be implemented purely in Go?
- What requires platform-specific terminal, process, or filesystem support?
- What can be tested in CI without a human terminal?

## Suggested Priority For Study

1. POSIX non-interactive scripts: establishes semantic correctness and testable
   core behavior.
2. Cross-platform execution substrate: determines what Windows native, MSYS2,
   Cygwin, and WSL can realistically support.
3. Common script extensions: determines compatibility expectations before a
   parser/runtime strategy is chosen.
4. Interactive shell loop and line editing: determines architecture for terminal
   state, completion, history, and prompt.
5. zsh-specific richness: determines long-term differentiators after the core is
   reliable.

## Output Table

Maintain a capability table:

```text
Domain | POSIX baseline | zsh behavior | bash/ksh/dash comparison | Platform constraints | Candidate tests | Design questions
```

The table should state behavior and constraints only. Implementation choices are
reserved for a later design document.
