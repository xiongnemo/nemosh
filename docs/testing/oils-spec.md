# The Oils spec suite: how far nemosh is from bash

This page is written from `tests/oils/baseline.json` and `tests/oils/calibration.json`
by `NEMOSH_OILS=update`, and a test fails when it is out of date. What the suite is, and
why none of its cases says what nemosh must do, is in `tests/oils/README.md`.

Measured on Windows, with the cases of Oils `15de8fd` (2026-05-30), against

- bash: GNU bash, version 5.3.15(2)-release (x86_64-pc-cygwin)
- busybox: BusyBox v1.38.0-FRP-6075-g169694ebd (2026-05-06 12:57:04 UTC)

**nemosh passes 2085 of the 2548 cases bash passes: 81.8%.**

Of the other 463, it does 108 the way the files record of ash, which may be busybox's
way, and 355 neither way.

| | cases |
|---|---:|
| in the vendored files | 2781 |
| in files Oils itself does not run | 46 |
| left out on Windows | 70 |
| measured | 2665 |
| that bash passes | 2548 |
| that nemosh passes of those | 2085 |

Left out on Windows, as cases no shell can be measured on there, for the reasons
`tests/oils/exclusions.json` gives:

- /proc: 3
- HOME: 1
- descriptors past 2: 4
- executable bits: 34
- resource limits: 17
- symbolic links: 7
- the prompt's symbol for an administrator: 3
- writing at the root: 1

## File by file

Of each file's measured cases: how many bash passes; how many of those nemosh passes,
and does the way ash does instead; and how many busybox does the way the file records
of ash.

| file | measured | bash | nemosh | % | as ash | busybox |
|---|---:|---:|---:|---:|---:|---:|
| alias.test.sh | 48 | 47 | 39 | 83.0% | 0 | 42 |
| append.test.sh | 20 | 20 | 20 | 100.0% | 0 | 1 |
| arg-parse.test.sh | 3 | 3 | 1 | 33.3% | 2 | 3 |
| arith-context.test.sh | 16 | 16 | 11 | 68.8% | 0 | 4 |
| arith-dynamic.test.sh | 4 | 4 | 1 | 25.0% | 0 | 1 |
| arith.test.sh | 74 | 74 | 60 | 81.1% | 1 | 44 |
| array-assign.test.sh | 9 | 9 | 6 | 66.7% | 0 | 9 |
| array-assoc.test.sh | 42 | 36 | 35 | 97.2% | 0 | 0 |
| array-basic.test.sh | 5 | 5 | 5 | 100.0% | 0 | 0 |
| array-compat.test.sh | 12 | 12 | 11 | 91.7% | 0 | 1 |
| array-literal.test.sh | 19 | 16 | 16 | 100.0% | 0 | 0 |
| array-sparse.test.sh | 40 | 40 | 27 | 67.5% | 0 | 0 |
| array.test.sh | 78 | 78 | 71 | 91.0% | 2 | 7 |
| assign-deferred.test.sh | 9 | 9 | 7 | 77.8% | 0 | 1 |
| assign-dialects.test.sh | 4 | 4 | 4 | 100.0% | 0 | 0 |
| assign-extended.test.sh | 39 | 34 | 25 | 73.5% | 1 | 0 |
| assign.test.sh | 48 | 45 | 37 | 82.2% | 1 | 28 |
| background.test.sh | 27 | 25 | 24 | 96.0% | 1 | 18 |
| ble-features.test.sh | 9 | 9 | 4 | 44.4% | 3 | 9 |
| ble-idioms.test.sh | 26 | 26 | 21 | 80.8% | 2 | 23 |
| ble-unset.test.sh | 5 | 5 | 0 | 0.0% | 5 | 5 |
| blog1.test.sh | 9 | 9 | 8 | 88.9% | 1 | 8 |
| blog2.test.sh | 8 | 8 | 3 | 37.5% | 0 | 2 |
| bool-parse.test.sh | 8 | 8 | 8 | 100.0% | 0 | 8 |
| brace-expansion.test.sh | 55 | 54 | 50 | 92.6% | 3 | 8 |
| bugs.test.sh | 28 | 28 | 20 | 71.4% | 7 | 28 |
| builtin-bash.test.sh | 13 | 13 | 11 | 84.6% | 0 | 10 |
| builtin-bind.test.sh | 9 | 8 | 0 | 0.0% | 0 | 0 |
| builtin-bracket.test.sh | 47 | 45 | 39 | 86.7% | 0 | 36 |
| builtin-cd.test.sh | 25 | 22 | 12 | 54.5% | 2 | 11 |
| builtin-completion.test.sh | 51 | 50 | 39 | 78.0% | 0 | 0 |
| builtin-dirs.test.sh | 18 | 18 | 7 | 38.9% | 0 | 2 |
| builtin-echo.test.sh | 27 | 27 | 26 | 96.3% | 1 | 27 |
| builtin-eval-source.test.sh | 23 | 21 | 14 | 66.7% | 0 | 13 |
| builtin-fc.test.sh | 14 | 14 | 1 | 7.1% | 1 | 2 |
| builtin-getopts.test.sh | 31 | 31 | 24 | 77.4% | 7 | 31 |
| builtin-history.test.sh | 17 | 15 | 4 | 26.7% | 0 | 1 |
| builtin-kill.test.sh | 20 | 17 | 15 | 88.2% | 0 | 11 |
| builtin-meta-assign.test.sh | 11 | 11 | 11 | 100.0% | 0 | 11 |
| builtin-meta.test.sh | 15 | 15 | 9 | 60.0% | 1 | 13 |
| builtin-misc.test.sh | 7 | 5 | 3 | 60.0% | 0 | 4 |
| builtin-printf.test.sh | 63 | 55 | 48 | 87.3% | 7 | 57 |
| builtin-process.test.sh | 10 | 9 | 7 | 77.8% | 0 | 6 |
| builtin-read.test.sh | 64 | 63 | 57 | 90.5% | 4 | 60 |
| builtin-set.test.sh | 24 | 24 | 20 | 83.3% | 0 | 20 |
| builtin-special.test.sh | 12 | 11 | 7 | 63.6% | 2 | 10 |
| builtin-times.test.sh | 1 | 1 | 1 | 100.0% | 0 | 1 |
| builtin-trap-bash.test.sh | 23 | 23 | 18 | 78.3% | 0 | 4 |
| builtin-trap-err.test.sh | 22 | 22 | 19 | 86.4% | 2 | 22 |
| builtin-trap.test.sh | 33 | 33 | 18 | 54.5% | 4 | 25 |
| builtin-type-bash.test.sh | 21 | 21 | 13 | 61.9% | 1 | 0 |
| builtin-type.test.sh | 4 | 4 | 3 | 75.0% | 1 | 4 |
| builtin-umask.test.sh | 24 | 15 | 9 | 60.0% | 0 | 2 |
| builtin-vars.test.sh | 41 | 39 | 31 | 79.5% | 1 | 23 |
| case_.test.sh | 13 | 12 | 12 | 100.0% | 0 | 9 |
| command-parsing.test.sh | 5 | 5 | 5 | 100.0% | 0 | 5 |
| command-sub.test.sh | 30 | 28 | 27 | 96.4% | 1 | 28 |
| command_.test.sh | 8 | 6 | 3 | 50.0% | 0 | 4 |
| comments.test.sh | 2 | 2 | 2 | 100.0% | 0 | 2 |
| dbracket.test.sh | 49 | 49 | 31 | 63.3% | 0 | 19 |
| divergence.test.sh | 3 | 3 | 3 | 100.0% | 0 | 3 |
| dparen.test.sh | 15 | 14 | 13 | 92.9% | 0 | 0 |
| empty-bodies.test.sh | 3 | 3 | 1 | 33.3% | 2 | 1 |
| errexit-osh.test.sh | 35 | 35 | 32 | 91.4% | 0 | 35 |
| errexit.test.sh | 35 | 34 | 31 | 91.2% | 3 | 35 |
| exit-status.test.sh | 11 | 11 | 10 | 90.9% | 0 | 7 |
| explore-parsing.test.sh | 5 | 5 | 4 | 80.0% | 0 | 4 |
| extglob-files.test.sh | 23 | 23 | 18 | 78.3% | 0 | 0 |
| extglob-match.test.sh | 29 | 29 | 28 | 96.6% | 0 | 0 |
| fatal-errors.test.sh | 5 | 5 | 0 | 0.0% | 3 | 3 |
| for-expr.test.sh | 9 | 8 | 7 | 87.5% | 0 | 0 |
| func-parsing.test.sh | 15 | 15 | 12 | 80.0% | 3 | 13 |
| glob-bash.test.sh | 8 | 8 | 8 | 100.0% | 0 | 8 |
| glob.test.sh | 39 | 37 | 33 | 89.2% | 0 | 36 |
| globignore.test.sh | 18 | 17 | 16 | 94.1% | 0 | 1 |
| globstar.test.sh | 4 | 4 | 4 | 100.0% | 0 | 0 |
| here-doc.test.sh | 32 | 32 | 31 | 96.9% | 1 | 32 |
| if_.test.sh | 5 | 5 | 5 | 100.0% | 0 | 5 |
| interactive.test.sh | 18 | 17 | 12 | 70.6% | 0 | 0 |
| introspect.test.sh | 13 | 12 | 10 | 83.3% | 0 | 2 |
| known-differences.test.sh | 2 | 2 | 1 | 50.0% | 1 | 2 |
| let.test.sh | 2 | 2 | 2 | 100.0% | 0 | 1 |
| loop.test.sh | 29 | 28 | 23 | 82.1% | 2 | 23 |
| nameref.test.sh | 32 | 32 | 28 | 87.5% | 0 | 2 |
| nix-idioms.test.sh | 6 | 6 | 4 | 66.7% | 0 | 0 |
| nocasematch-match.test.sh | 6 | 6 | 6 | 100.0% | 0 | 3 |
| nul-bytes.test.sh | 16 | 16 | 14 | 87.5% | 1 | 12 |
| paren-ambiguity.test.sh | 8 | 8 | 6 | 75.0% | 2 | 8 |
| parse-errors.test.sh | 27 | 25 | 19 | 76.0% | 2 | 22 |
| pipeline.test.sh | 26 | 26 | 25 | 96.2% | 0 | 17 |
| posix.test.sh | 15 | 15 | 14 | 93.3% | 0 | 14 |
| print-source-code.test.sh | 4 | 4 | 4 | 100.0% | 0 | 0 |
| process-sub.test.sh | 9 | 9 | 9 | 100.0% | 0 | 5 |
| prompt.test.sh | 31 | 24 | 22 | 91.7% | 0 | 0 |
| quote.test.sh | 35 | 35 | 34 | 97.1% | 1 | 35 |
| redirect-command.test.sh | 23 | 23 | 22 | 95.7% | 0 | 22 |
| redirect-multi.test.sh | 13 | 13 | 5 | 38.5% | 2 | 7 |
| redirect.test.sh | 39 | 37 | 33 | 89.2% | 1 | 28 |
| regex.test.sh | 37 | 36 | 33 | 91.7% | 0 | 10 |
| serialize.test.sh | 10 | 9 | 8 | 88.9% | 1 | 10 |
| sh-func.test.sh | 12 | 12 | 10 | 83.3% | 0 | 10 |
| sh-options-bash.test.sh | 9 | 9 | 7 | 77.8% | 0 | 0 |
| sh-options.test.sh | 39 | 39 | 31 | 79.5% | 0 | 12 |
| sh-usage.test.sh | 23 | 22 | 20 | 90.9% | 0 | 18 |
| smoke.test.sh | 18 | 18 | 16 | 88.9% | 0 | 16 |
| strict-options.test.sh | 17 | 15 | 12 | 80.0% | 1 | 7 |
| subshell.test.sh | 2 | 2 | 2 | 100.0% | 0 | 2 |
| temp-binding.test.sh | 4 | 3 | 2 | 66.7% | 1 | 4 |
| tilde.test.sh | 14 | 12 | 11 | 91.7% | 1 | 10 |
| toysh-posix.test.sh | 22 | 22 | 15 | 68.2% | 7 | 22 |
| toysh.test.sh | 8 | 7 | 6 | 85.7% | 0 | 1 |
| type-compat.test.sh | 7 | 5 | 4 | 80.0% | 0 | 0 |
| unicode.test.sh | 7 | 2 | 1 | 50.0% | 0 | 2 |
| var-num.test.sh | 5 | 5 | 5 | 100.0% | 0 | 5 |
| var-op-bash.test.sh | 27 | 26 | 25 | 96.2% | 0 | 2 |
| var-op-len.test.sh | 9 | 5 | 3 | 60.0% | 1 | 3 |
| var-op-patsub.test.sh | 28 | 28 | 25 | 89.3% | 1 | 21 |
| var-op-slice.test.sh | 22 | 22 | 19 | 86.4% | 0 | 6 |
| var-op-strip.test.sh | 29 | 29 | 29 | 100.0% | 0 | 27 |
| var-op-test.test.sh | 37 | 37 | 32 | 86.5% | 1 | 20 |
| var-ref.test.sh | 30 | 30 | 20 | 66.7% | 0 | 0 |
| var-sub-quote.test.sh | 41 | 41 | 40 | 97.6% | 0 | 37 |
| var-sub.test.sh | 6 | 6 | 4 | 66.7% | 1 | 4 |
| vars-bash.test.sh | 1 | 1 | 1 | 100.0% | 0 | 0 |
| vars-special.test.sh | 41 | 39 | 28 | 71.8% | 0 | 19 |
| whitespace.test.sh | 5 | 0 | 0 | - | 0 | 0 |
| word-eval.test.sh | 8 | 7 | 7 | 100.0% | 0 | 6 |
| word-split.test.sh | 55 | 53 | 47 | 88.7% | 5 | 53 |
| xtrace.test.sh | 19 | 17 | 11 | 64.7% | 0 | 9 |
| zsh-idioms.test.sh | 3 | 3 | 2 | 66.7% | 0 | 2 |
| all | 2665 | 2548 | 2085 | 81.8% | 108 | 1447 |
