# Nemosh Research Index

This directory captures the research needed before designing or implementing a
Go-based cross-platform shell with POSIX grounding and a long-term zsh-level
capability target.

The documents are research artifacts, not implementation specs. They should
help later design work answer four questions:

- Which standard behavior must be treated as normative?
- Which existing shells and compatibility layers should be used as references?
- Which test suites can be reused, and what do they actually prove?
- Which platform and CI constraints must be understood before writing runtime code?

## Reading Order

1. `posix-standard-map.md` maps the relevant POSIX.1-2024 areas and the main
   semantic risks for a shell implementation.
2. `zsh-capability-map.md` decomposes the long-term zsh-level ambition into
   researchable capability areas.
3. `reference-implementations.md` lists reference projects to clone and the
   behavior questions to study in each.
4. `reference-clone-status.md` records the current local shallow clones and
   their approximate sizes.
5. `behavior-matrix.md` records the first architecture-driving behavior matrix
   across local references.
6. `test-suite-survey.md` surveys reusable test suites and explains the limits
   of open-source conformance testing versus official certification.
7. `github-actions-feasibility.md` sketches the CI research matrix for Linux,
   macOS, native Windows, MSYS2, Cygwin, and WSL experiments.
8. `decision-notes.md` records user-confirmed direction for the next research
   and design pass.
9. `research-findings.md` records facts already verified in the first research
   pass and near-term follow-up tasks.
10. `autocomplete-feasibility.md` evaluates post-v0 autocomplete prerequisites,
    architecture, Windows constraints, editor options, and staged experiments.
11. `open-questions.md` records decisions that should wait until after the
    research phase.

## Research Workspace Convention

When the research phase is executed, clone references outside production code,
for example:

```text
references/
  shells/
  go-shells/
  windows-compat/
```

Use shallow clones first unless history is needed for a specific question. Keep
local patches out of cloned references; write observations in these research
documents instead.

## Traceability Rule

Every durable conclusion should point to at least one source category:

- POSIX standard text
- Reference implementation source
- Reference implementation test suite
- Platform or CI documentation
- A reproducible behavior experiment

Avoid treating folklore as a design input. If a behavior cannot be traced, add
it to `open-questions.md` or create a small experiment for it.

## Design Drafts

- `../design/v0-scope.md` records the current implementation scope for v0.
- `../design/v0-readiness.md` maps that authoritative scope to current
  implementation and test evidence, blockers, and ordered acceptance waves.
- `../design/pre-implementation-plan.md` records the checklist and milestones to
  complete before production Go runtime code starts.
- `../design/windows-path-model.md` records the current Windows path model draft.
- `../design/windows-execution-model.md` records the current Windows command
  lookup and process-launch draft.

## Testing Drafts

- `../testing/behavior-test-format.md` records the project-owned behavior test
  metadata format.
- `../testing/initial-behavior-cases.md` lists the first POSIX, busybox-w32,
  Windows, and Nemosh-specific behavior gates.
- `../testing/applet-test-inventory.md` lists applet milestones and the rule that
  every implemented applet needs tests.
