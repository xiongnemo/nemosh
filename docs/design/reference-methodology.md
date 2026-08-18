# How Nemosh Uses Its Reference Implementations

Nemosh is licensed Apache-2.0. BusyBox and busybox-w32 are GPL-2.0. This
document states exactly what is taken from them, because "we looked at busybox"
is a claim that deserves to be precise rather than left to inference.

## What is taken

**Behaviour, and only behaviour.** What a command does: which options it
accepts, what it writes, what status it exits with, which order it does things
in, and where it refuses. These are facts about a program's operation. Copyright
does not reach them — 17 U.S.C. §102(b) excludes "any idea, procedure, process,
system, method of operation" from protection, whatever form it is described in.

Behaviour is what a compatible shell has to reproduce. A `find` that returns
different paths than every other `find` is not a `find`.

## What is not taken

**No code.** Not a function, not a loop, not a data structure, not a build
rule. Nemosh is written in Go against Go's standard library and two BSD-licensed
`golang.org/x` packages. There is no translation step and no file with a
counterpart.

**Nothing is distributed.** The reference clones live under `references/`, which
is in `.gitignore` and has never been committed. Verified before the repository
was made public: `git log --all --name-only` reports no path under
`references/` in any commit. GPL-2.0 obligations attach to distribution, and
Nemosh distributes no BusyBox bytes.

## How a behaviour is established

The rule in `AGENTS.md` is that a claim is measured, not remembered. In
practice that means running the reference and recording what came back:

```console
$ busybox find . -name '*.txt'
./f.txt
./m.txt
./sub/deep.txt
```

Source is read to find *where* a behaviour is decided, so the reading can be
checked by someone else — `shell/ash.c:12050` says `fg` sits under `#if JOBS`,
which is why Nemosh refuses it rather than guessing that Windows cannot support
it. The citation is a pointer to evidence, not a record of copying. Every one of
the 137 file-and-line citations in this repository is of that kind.

Where the behaviour is then implemented differently, the comment says so.
`set -o nocaseglob` cites busybox's `FNM_CASEFOLD` (`shell/ash.c:9230`) and
folds both sides with `strings.ToLower`. The line editor cites
`libbb/lineedit.c` in order to *not* repeat its defect: busybox's backspace
moves one column and deletes one character, so a two-column CJK character loses
half of itself, and Nemosh edits by rune while measuring by column.

## Test material

A test file is source. `testsuite/tr.tests` opens with
`Copyright 2009 by Denys Vlasenko` and `Licensed under GPLv2`, exactly as
`coreutils/tr.c` does, and copying one is the same act as copying the other.

It is worth stating separately because the intuition runs the other way — a test
looks like a description of behaviour rather than an implementation of it. Two
reasons it is not safer:

- Almost all of a test file is material that can be copied verbatim: the input
  strings, the expected output, the order. There is no "write it your own way"
  step to put distance in.
- **The selection and arrangement of cases is its own authorship.** The
  individual fact each case asserts is not protected, but a curated list of forty
  edge cases for `tr` is a compilation, and compilations are the one place where
  arranging unprotected facts can attract protection.

So the rule is the same as for source, and the escape hatch is better than for
source: **run the binary**. The differential suite executes the reference and
records what came back, which yields facts observed here rather than transcribed
from there. A reference test is a legitimate *lead* — it says where to probe —
and the expectation that goes into `tests/behavior/` is then measured, never
copied across.

Checked on 2026-08-15: nothing in this repository carries busybox's test idioms
(`. ./testing.sh`, `testing "…"`, `optional FEATURE_…`, `SKIP=`); the search
returns no files outside the ignored `references/` clones.

## Wording

Error text is behaviour a script can branch on, so its *shape* is matched: which
operand is quoted, what the status is, whether a hint follows. The *phrasing* is
not taken where it is the reference's own invention:

| Nemosh | busybox | why |
| --- | --- | --- |
| `cannot open 'x'` | `can't open 'x'` | The contraction is busybox's; GNU coreutils writes `cannot`. Carries no behaviour. |
| `unsupported expression: -mtime` | `unrecognized: -mtime` | Matches Nemosh's own vocabulary, which says `unsupported <applet> option` elsewhere. |

Both are recorded as declared divergences in the behavior corpus, so the
differential runner reports them instead of being surprised by them.

Where wording *is* identical it is because it is not anyone's invention:
`No such file or directory` is the C library's `strerror(ENOENT)`, and
`not found` is what every shell has said since the Bourne shell.

## What this is not

This is **not** a clean-room reimplementation in the strict sense. A strict
clean room has one group read the original and write a specification, and a
second group implement from that specification without seeing the original. That
is not what happened here: the same author read busybox and wrote the Go.

Saying so plainly is the point. The defensible position is that behaviour is not
protected expression and none was copied — not that nobody looked.

## Reference order

1. **busybox-w32** for native Windows behaviour and its tradeoffs.
2. **BusyBox ash** for shell and runtime structure.
3. **dash** and **POSIX** for portable `sh` behaviour.

`bash` is consulted for interactive conventions its own `bind -q` reports, such
as the readline key bindings, because those are the conventions users have in
their fingers.

## Public test suites this project may use

Behaviour is not protected expression, but a test file is source like any other,
which is the whole reason this document exists. So the question "can we borrow a
conformance suite" is a licence question, not a behaviour question, and the answer
differs sharply between the obvious candidates.

Nemosh is Apache-2.0. Inbound-compatible means permissive: Apache-2.0, BSD, ISC,
MIT. A GPL suite is not usable *as files* no matter how useful it would be.

Each licence below was checked against the project's own licence file or a source
header on 2026-08-18, not recalled.

| suite | licence | what it is good for |
| --- | --- | --- |
| **Oils** `spec/*.test.sh` | Apache-2.0 (`LICENSE.txt`, repo-wide; no separate grant under `spec/`) | The shell language. Thousands of cases, and the format already records *which shells disagree* |
| **toybox** `tests/*.test` | ISC (`LICENSE`) | The applets — the coreutils surface, which is where 58 of ours live |
| **FreeBSD** `bin/sh/tests` | BSD-3-Clause (from `bin/sh` headers; the test files carry none of their own) | POSIX `sh` conformance, written by people maintaining a real `sh` |

Not usable, and worth naming so nobody spends an afternoon on them:

- **bash**'s own tests — GPL-3.0.
- **busybox**'s `testsuite/` — GPL-2.0. Doubly awkward, since busybox-w32 is the
  primary *behaviour* reference: we may read it to learn what it does and may not
  copy its test files. That distinction is exactly the one this document draws.
- **GNU coreutils** `tests/` — GPL-3.0.

Oils is the best fit and it is worth saying why beyond the licence. Its case
format carries a `## compare_shells: bash dash mksh zsh` header and per-shell
expected values, because it was built to describe where shells legitimately
differ. That is the same shape as this repository's `[differential]` table with
its required `why`. A case imported from there arrives with the knowledge of which
reference disagrees already attached, which is the expensive half of writing one.

Anything imported must carry attribution in `THIRD-PARTY-NOTICES.md` naming the
suite, its licence, and the commit it came from, and must be translated into this
repository's own case format rather than vendored as a runnable script — a
`spec/*.test.sh` runner would be a second test framework, and the corpus already
has one.
