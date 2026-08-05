# Autocomplete Feasibility And Implementation Plan

## Status And Conclusion

**Fact.** Autocomplete is explicitly outside v0: `docs/design/v0-scope.md:151-154`
defers "full REPL polish, completion, plugins, and zsh-level interaction." The
current command REPL is line-oriented: `cmd/nemosh/main.go:86-126` reads complete
newline-delimited strings with `bufio.Reader.ReadString`, and its tests provide
ordinary `io.Reader` input rather than terminal key or cursor events
(`cmd/nemosh/interactive_test.go:18-35,187-208`).

**Conclusion.** Autocomplete is feasible after v0, but it requires a terminal
editor boundary and several reusable runtime APIs. The recommended architecture
is a **Nemosh-owned semantic completion engine behind an editor adapter**. The
engine owns parsing, replacement ranges, Windows path and command semantics,
providers, ranking, cancellation, and caching. A line-editor dependency, if
adopted, owns input and rendering only.

**Recommendation.** Prototype both `nyaosorg/go-readline-ny` and
`reeflective/readline`. Use `joeycumines/go-prompt` as an API and interaction
benchmark. Do not select or add a dependency until an empirical Windows-focused
bakeoff passes the matrix in this document. `golang.org/x/term` remains useful
for terminal primitives or a minimal fallback, not a complete completion stack.

**No implementation was performed.** This artifact introduces no autocomplete
code or dependency and does not change the v0 scope.

## Evidence And Current Seams

### Reusable foundations

- **Fact:** `internal/applets/process_view.go:12-33` exposes working directory,
  environment, environment lookup, and path resolution. The runtime implements
  that view (`internal/shell/runtime/execution_state.go:23-36,72-80`) and injects
  it into applets (`internal/shell/runtime/runtime.go:175-180`). This is a useful
  immutable-request input seam, but it does not enumerate files or commands.
- **Fact:** command execution already has PATH traversal and deterministic
  Windows suffix policy in `internal/shell/runtime/external.go:14-76`, with
  behavior covered by `internal/shell/runtime/runtime_external_test.go:60-174`.
- **Fact:** `internal/pathmodel/model.go:36-153` models aliases, virtual roots,
  drive and UNC forms, while `internal/shell/runtime/path.go:8-20` has a smaller
  runtime resolver. Completion must not create a third interpretation.
- **Fact:** the Windows design requires canonical `/c/...` display, accepted
  `/c`, `/mnt/c`, drive, quoted/backslash, and UNC input forms, and no general
  external argv conversion (`docs/design/windows-path-model.md:15-90,127-149`).
  Lookup order, suffixes, scripts, PATH, Unicode, and long paths are specified in
  `docs/design/windows-execution-model.md:14-129,178-195`.

### Missing prerequisites

- **Fact:** the current REPL has no key-event, editable-buffer, cursor-position,
  candidate-menu, or redraw contract. A line editor cannot be slipped into the
  existing `ReadString` loop without replacing that input boundary.
- **Fact:** `internal/applets/registry.go:15-30,41-81` keeps names in a private
  map and exports lookup but not enumeration. `internal/appletmanifest` parses
  source for tooling; it is not a runtime enumeration API.
- **Fact:** external command resolver helpers are package-private
  (`internal/shell/runtime/external.go:14-18,35-76`). A completion package cannot
  reuse them directly, and duplicating their suffix and PATH rules would allow
  suggestions that execution resolves differently.
- **Recommendation:** before rich completion, expose side-effect-free,
  test-covered APIs for command enumeration/resolution and registry names, and
  converge runtime path resolution with `internal/pathmodel` behind one policy.
- **Open question:** whether those APIs live in runtime, a lower-level resolver
  package, or a read-only shell snapshot should be settled during design; they
  should not expose mutable runtime internals.

## Proposed Architecture

```text
line editor
  -> editor adapter (native cursor units <-> snapshot byte ranges)
  -> request builder and shell-aware cursor analysis
  -> provider coordinator
       command | path | variable | syntax/keyword | later command-specific
  -> normalization, filtering, ranking, deduplication
  -> insertion plan and display candidates
  -> editor adapter
```

The adapter is deliberately thin. It may translate key bindings, buffer state,
cursor units, menus, resize, and redraw events, but it must not decide shell
quoting, Windows executable rules, or candidate meaning. This keeps editor
replacement and library migration local.

### Immutable request snapshot

Every request should capture one immutable view:

```text
Request
  generation             monotonically increasing identifier
  buffer_utf8             immutable command-line string
  cursor_byte             checked UTF-8 byte boundary in buffer_utf8
  cwd                     shell working directory
  environment             shell environment snapshot or immutable view/version
  path_value              exact shell PATH value used by execution
  shell_state_version     functions, builtins/applets, variables, options
  platform/path_policy    Windows versus POSIX and active path aliases
  trigger                 explicit Tab, menu refresh, or later autosuggestion
  budget/deadline         coordinator and provider limits
```

Providers return results tied to that generation and snapshot. Publication and
insertion are allowed only if the active generation and relevant buffer identity
still match. A replacement byte range is valid only for its original immutable
UTF-8 snapshot; it must never be retained across buffer mutation.

### Text and cursor units

The design must name and convert four distinct units:

| Unit | Use |
| --- | --- |
| Byte offset | Parser spans and replacement slices in an immutable Go string |
| Rune offset | Code-point-oriented editor buffer APIs |
| Grapheme boundary | User-perceived cursor movement and deletion, including combining and ZWJ sequences |
| Display cell/column | Terminal cursor placement, wrapping, and menu alignment |

A UTF-8 rune can occupy several bytes; a grapheme can contain several runes; a
grapheme can occupy zero, one, two, or terminal-dependent cells. ANSI control
sequences occupy bytes but normally no cells. Adapter conversions must be
checked against the same snapshot, and rendering must not use byte or rune count
as terminal width.

This separation is directly relevant because `golang.org/x/term` documents its
autocomplete `pos` as a byte index, while the evaluated editors use rune-based
buffer positions in at least part of their APIs. See the pinned
[`x/term` callback source](https://github.com/golang/term/blob/9f69229da31ca6a34b522f59dbe07cad5ea21587/terminal.go#L55-L78)
and the official [`x/term` API](https://pkg.go.dev/golang.org/x/term).

### Candidate and insertion schema

The semantic engine should return structured candidates, not preformatted menu
strings:

```text
Candidate
  provider_id
  kind                    command, applet, function, path, directory, variable, ...
  insert_text             logical text before quote/escape adaptation
  display_text            user-facing spelling, possibly canonical `/c/...`
  description             optional, plain text
  replace_start_byte      snapshot-relative inclusive boundary
  replace_end_byte        snapshot-relative exclusive boundary
  append_space            policy hint, false for directories by default
  append_separator        policy hint for directories
  attributes              directory, hidden, system, reparse point, executable source
  score/group             stable ranking and grouping inputs
```

Insertion is a separate operation that understands unquoted, single-quoted,
double-quoted, assignment, and command-position contexts. It computes escaped
text, trailing separator/space, and the new cursor location. Fish is evidence
for keeping candidate computation, insertion semantics, common-prefix behavior,
and pager state separate, not a requirement to clone fish behavior.

### Initial providers

1. **Command provider:** shell functions and builtins when available, applet
   names, then external commands. It must reuse the execution lookup order and
   fixed Windows suffix policy rather than independently interpreting `PATH`.
2. **Path provider:** classify explicit path form, resolve relative to snapshot
   cwd through the shared path model, enumerate one directory, preserve the
   user's accepted input spelling where practical, and emit canonical display
   spelling according to project policy.
3. **Variable provider:** enumerate the immutable shell environment/state and
   respect `$name` versus `${name}` insertion context.
4. **Syntax provider:** a small, parser-backed set of context-valid reserved
   words/operators only after cursor analysis can do so without executing input.
5. **Later providers:** command-specific options, history, and plugins remain
   out of the first implementation slice.

Providers must be read-only. Completion must never source a script, execute a
candidate, load an untrusted plugin, perform shell expansion with side effects,
or probe a path by launching it.

## Windows-Specific Requirements

### Path forms and display

Completion must distinguish, not flatten, relative paths, root-relative paths,
drive-absolute `C:\\x`, drive-relative `C:x`, `/c/x`, `/mnt/c/x`, UNC
`\\server\share\x`, and extended-length paths. Microsoft documents these forms
and their non-equivalence in [Naming Files, Paths, and
Namespaces](https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file)
and [Maximum Path Length
Limitation](https://learn.microsoft.com/en-us/windows/win32/fileio/maximum-file-path-limitation).
Nemosh policy remains controlled by `docs/design/windows-path-model.md`, not by
blind separator replacement.

The provider should retain a typed parsed form, filesystem lookup form, insertion
form, and display form. UNC completion starts only after a valid share boundary;
network-host/share discovery is outside the initial provider. Extended-length
paths are an API-boundary concern and should not leak into ordinary display.

### Attributes and visibility

Windows hidden state is metadata, not a leading-dot convention. Candidate
records should retain `FILE_ATTRIBUTE_HIDDEN`, `FILE_ATTRIBUTE_SYSTEM`,
`FILE_ATTRIBUTE_DIRECTORY`, and `FILE_ATTRIBUTE_REPARSE_POINT` when available;
definitions come from Microsoft's [File Attribute
Constants](https://learn.microsoft.com/en-us/windows/win32/fileio/file-attribute-constants).

**Recommendation:** explicit completion may include hidden/system candidates
only under a documented policy (for example, when the typed prefix already
matches); ordinary menus should avoid surprising disclosure/noise. Dotfiles on
Windows still participate by name and are not automatically equivalent to the
hidden attribute.

### Command lookup and security

Suggestions in command position must be derived from the same resolver policy
used at execution: functions/special builtins/builtins, applets, then PATH, with
the fixed Windows suffix order and explicit script boundaries documented in
`docs/design/windows-execution-model.md`. Microsoft `CreateProcessW` is relevant
platform evidence but not a full shell lookup specification. Its distinction
between application name and command line also makes quoting paths with spaces
security-sensitive; see [`CreateProcessW`](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessw).

Never mark a candidate executable from POSIX mode bits or the Windows read-only
attribute. Never search the current directory implicitly unless Nemosh execution
does. Deduplicate PATH results by the same case policy execution uses, preserve
the selected display spelling, and never interpolate candidate descriptions as
terminal control sequences. Sanitize or escape control characters in names and
descriptions before rendering.

## Performance, Cancellation, And Caching

The input loop must remain responsive even when a provider is slow.

- Assign every request a generation; only the current generation may publish.
- Cancel the previous request context when a new generation starts. Providers
  that accept context should check it during loops and before expensive work.
- Treat cancellation and staleness separately. A library or OS call that cannot
  stop may finish in the background, but its stale result is discarded. Bound
  such work so obsolete producers cannot accumulate or block the editor.
- Use a coordinator deadline plus per-provider budgets. The first prototype
  should measure before fixing targets; a reasonable bakeoff gate is no input
  stall, fast local results targeted within 50 ms, and an initial menu within
  100 ms. These are recommendations to validate, not current guarantees.
- Cap candidates and directory/PATH entries scanned per request. Return a
  truncated marker rather than allocating an unbounded menu.
- Use bounded, nonblocking result delivery and panic/error isolation at provider
  boundaries. Merge partial results deterministically.
- Cache directory listings by resolved directory plus platform-relevant identity
  and short TTL; cache external command sets by cwd, exact PATH, suffix policy,
  and registry/shell-state version. Never cache a replacement span independently
  of its snapshot. Bound cache entries and bytes; invalidate conservatively.
- Avoid recursive filesystem walks, network share discovery, file-content reads,
  and process execution in the initial providers.

Fish at commit
[`2a28107be769cd79b28793889f440ba19da78c3e`](https://github.com/fish-shell/fish-shell/commit/2a28107be769cd79b28793889f440ba19da78c3e)
provides useful reference points for request deduplication and stale-result
handling ([reader source](https://github.com/fish-shell/fish-shell/blob/2a28107be769cd79b28793889f440ba19da78c3e/src/reader/reader.rs#L5527-L5603)),
plus cancellation-aware wildcard expansion
([source](https://github.com/fish-shell/fish-shell/blob/2a28107be769cd79b28793889f440ba19da78c3e/src/reader/reader.rs#L6575-L6615)).
This supports generation checks and cooperative cancellation where available; it
does not prove that every provider can be preempted.

## Editor Library Comparison

The source review uses immutable snapshots. It evaluates exposed mechanics, not
project popularity or an unsupported promise of complete Windows compatibility.

| Option | Evidence-backed strengths | Constraints to test | Role |
| --- | --- | --- | --- |
| [`nyaosorg/go-readline-ny@5699958`](https://github.com/nyaosorg/go-readline-ny/tree/56999586c2f61d89d49bb73808d9b6f532024e67) | `ReadLine(context.Context)`, contexts on commands, rune-count insertion contract, explicit width/ZWJ machinery | Completion invocation is synchronous and the reviewed completion API itself has no context; verify Windows Terminal/console, menus, resize, and latency | Prototype shortlist |
| [`reeflective/readline@d3aedcb`](https://github.com/reeflective/readline/tree/d3aedcb78a338d37f719cd64bc97148d51efc94b) | Rune-based line model, separate inserted/display candidate text, common-prefix support, `uniseg.StringWidth` display measurement | Completer is synchronous without context; useful internals may not be public; verify public API and Windows behavior | Prototype shortlist |
| [`joeycumines/go-prompt@e8b9e8f`](https://github.com/joeycumines/go-prompt/tree/e8b9e8f9d7540efcd61426476a8d3a6e80f29c2c) | Typed rune positions, grapheme-oriented movement, completion UI and width-aware document calculations | Synchronous completer without context; verify terminal-width edge cases and adapter fit | API/UX benchmark |
| [`golang.org/x/term@9f69229`](https://github.com/golang/term/tree/9f69229da31ca6a34b522f59dbe07cad5ea21587) | Raw mode, terminal size/state, small terminal line reader, documented byte-index callback | Callback is per keypress, synchronous, context-free, and provides no candidate menu/provider orchestration | Primitive/fallback |

The bakeoff must implement the same thin adapter and Nemosh-owned fake provider
for each candidate. No editor may define semantic request or candidate types.

## Test And Experiment Matrix

Tests should follow the evidence and sandbox conventions in
`docs/testing/behavior-test-format.md`; unit tests should remain deterministic,
while interactive terminal experiments should record terminal and OS metadata.

| Area | Required cases | Platforms/evidence |
| --- | --- | --- |
| Cursor units | ASCII; UTF-8 multibyte; combining marks; emoji ZWJ; full-width characters; cursor mid-token; suffix after cursor | Unit/property tests plus Windows Terminal and representative Unix terminal |
| Parsing/insertion | Unquoted, single/double quoted, escaped spaces, empty token, assignment, command position, incomplete syntax, unique/common-prefix/ambiguous candidates | Engine unit and golden tests; no command execution |
| Windows paths | Relative/root-relative; `/c`; `/mnt/c`; `C:/`; quoted backslashes; `C:relative`; drive root; UNC share; long path boundary; invalid host-only UNC | Windows sandbox; compare shared path model and execution behavior |
| Attributes | Dot name; hidden attribute; system attribute; directory; file; reparse point; inaccessible entry | Windows filesystem fixture with explicit cleanup |
| Commands | Applet/builtin/function ordering; duplicate PATH names; relative and empty PATH entries; fixed `.com/.exe/.sh/.bat/.cmd` order; spaces; case variants; stale cache | Reuse runtime lookup fixtures and differential resolver assertions |
| Responsiveness | Slow/cancel-aware provider; slow/non-cancellable provider; rapid edits; repeated Tab; bounded result channel; candidate cap; resize while results arrive | Fake-clock/unit tests and PTY/console integration tests |
| Security | Control characters, hostile descriptions, quoted executable path, no implicit execution, inaccessible/network paths, cancellation under load | Unit/fuzz/sandbox tests; verify terminal state restoration |
| Editor bakeoff | Ctrl-C/Ctrl-D, history coexistence, multiline input, paste, resize, menu navigation, Unicode movement/deletion/alignment, terminal restoration after error | Windows Console, Windows Terminal, Linux/macOS terminal where CI permits |

Fuzz targets should include cursor-byte conversion, replacement-range validity,
quote adaptation, path classification, and candidate rendering. Invariants include
valid UTF-8 output, boundaries within the snapshot, no panic for incomplete shell
text, deterministic ordering, and no provider mutation of shell state.

## Staged Implementation Plan

These are post-v0 milestones; dates depend on prerequisite runtime stabilization.

1. **M0 - contracts and measurements (2-3 engineering days).** Freeze request,
   candidate, insertion, offset, visibility, and budget contracts. Build fixtures
   and measure current input behavior. Exit: reviewed design and test vectors;
   no dependency decision.
2. **M1 - reusable read-only seams (3-5 days).** Design and test applet/command
   enumeration, resolver reuse, immutable shell state, and one path policy. Exit:
   completion can query exactly what execution would resolve without executing.
3. **M2 - synchronous semantic engine (5-8 days).** Cursor analysis, path,
   command and variable providers, quoting/insertion, ranking, caps, and Unicode
   conversion tests behind a noninteractive harness. Exit: the core matrix passes
   without a terminal editor.
4. **M3 - editor bakeoff (4-6 days).** Implement equivalent adapters for
   `go-readline-ny` and `reeflective/readline`; benchmark against `go-prompt` API
   and `x/term` primitives. Record Windows console evidence, maintenance cost,
   and public-API gaps. Exit: written dependency decision or a justified minimal
   in-house/fallback direction.
5. **M4 - selected editor integration (4-7 days).** Replace the line-oriented
   interactive boundary while retaining noninteractive execution behavior. Add
   explicit completion, menus, resize, Ctrl-C, paste, and terminal restoration.
6. **M5 - asynchronous hardening (4-6 days).** Add generations, contexts,
   deadlines, stale-result rejection, bounded concurrency, metrics, and bounded
   caches. Exit: responsiveness/security matrix and stress tests pass.
7. **M6 - optional richness (separate proposals).** Command-specific providers,
   history suggestions, and plugins require their own scope and threat model.

## Risks And Open Questions

| Type | Item | Required resolution |
| --- | --- | --- |
| Risk | Completion and execution diverge because resolver/path logic is duplicated | Share read-only resolver and path policy before integration |
| Risk | Editor cursor units corrupt Unicode or replacement spans | Explicit conversion types and snapshot-bound ranges |
| Risk | Slow or uncancellable filesystem/API work stalls input or leaks goroutines | Budgets, bounded concurrency, stale rejection, stress tests |
| Risk | Windows terminal variants disagree on key events or emoji width | Bakeoff on real Console and Windows Terminal; document residual variance |
| Risk | Candidate names/descriptions inject control sequences or ambiguous commands | Sanitized display, semantic insertion, no execution, shared lookup policy |
| Open question | Exact hidden/system candidate visibility policy | Decide from Windows usability experiments and document it |
| Open question | Preserve typed path spelling or always canonicalize insertion | Test `/c`, drive, backslash, and UNC workflows; display and insertion may differ |
| Open question | Case matching and deduplication by provider/platform | Align command provider with execution and measure filesystem expectations |
| Open question | Public seam location for functions, registry, resolver, and state versions | Resolve in M1 without exposing mutable runtime internals |
| Open question | Whether either shortlisted editor supports required refresh/cancellation without a fork | Answer only through M3 bakeoff |

## Reference Sources

- Nemosh scope and policy: `docs/design/v0-scope.md`,
  `docs/design/windows-path-model.md`, and
  `docs/design/windows-execution-model.md`.
- Nemosh current seams: `cmd/nemosh/main.go`,
  `internal/applets/process_view.go`, `internal/applets/registry.go`,
  `internal/shell/runtime/external.go`, `internal/shell/runtime/path.go`, and
  `internal/pathmodel/model.go`.
- Fish snapshot
  [`2a28107be769cd79b28793889f440ba19da78c3e`](https://github.com/fish-shell/fish-shell/tree/2a28107be769cd79b28793889f440ba19da78c3e),
  especially completion application/orchestration in
  [`reader.rs`](https://github.com/fish-shell/fish-shell/blob/2a28107be769cd79b28793889f440ba19da78c3e/src/reader/reader.rs#L6668-L7185).
- Zsh snapshot
  [`489436767786ec8c8e16436d00ff3c7c4ce0a380`](https://github.com/zsh-users/zsh/tree/489436767786ec8c8e16436d00ff3c7c4ce0a380),
  especially [`_path_files`](https://github.com/zsh-users/zsh/blob/489436767786ec8c8e16436d00ff3c7c4ce0a380/Completion/Unix/Type/_path_files),
  as a path-completion complexity checklist rather than portable code. The
  pinned commit's glob-qualifier ordering fix is useful evidence that such rules
  are semantic, not a requirement to implement zsh syntax.
- Microsoft: [Naming Files, Paths, and
  Namespaces](https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file),
  [File Attribute
  Constants](https://learn.microsoft.com/en-us/windows/win32/fileio/file-attribute-constants),
  [Maximum Path Length
  Limitation](https://learn.microsoft.com/en-us/windows/win32/fileio/maximum-file-path-limitation),
  and [`CreateProcessW`](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessw).
- Evaluated editor and `x/term` immutable sources are linked in the comparison
  table above; their source APIs are inputs to the bakeoff, not dependency
  endorsements.
