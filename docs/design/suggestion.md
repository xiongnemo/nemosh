# Inline suggestion, and drawing the line

Grey text ahead of the cursor is not completion. Completion answers "what could
go here" when asked, and may take as long as reading a directory. A suggestion
answers "what did you probably mean" after **every keystroke**, and shows one
answer speculatively. fish draws it from history first and falls back to
completions.

That difference decides the whole design.

## Everything it consults is in memory

Nothing in `suggest.go` touches the filesystem, and that is a rule rather than an
accident. A directory read per keystroke is felt on a network drive or a cold
cache, and a shell that stutters as you type is worse than one that suggests
nothing.

Two sources, tried in order, which is the shape zsh gives its completers -- each
is a different guess and the first that answers wins:

1. **History**, most recent first. A line already run is a line meant, and it is
   the source fish leads with.
2. **Command names**, for the first word only. This is what history cannot help
   with in a fresh session, and a fresh session is exactly where a suggestion
   engine looks broken if it has nothing to say. The shortest match wins, so `e`
   suggests `echo` rather than whatever sorts first.

An operand is deliberately not suggested. That is a filesystem question and it
belongs to Tab, which is allowed to be slow because it was asked.

History is per-session today. Persisting it would make the first source much
stronger and is the single biggest improvement available to this feature.

## Three properties that make it safe to draw

**It is never in the buffer.** The engine returns the *remainder* -- what would be
added -- not the completed line, and the renderer draws it without ever storing
it. So Enter submits what was typed and could not do otherwise. That is a
property of the arrangement rather than a rule the key handler has to remember.

**It never wraps.** The suggestion is cut to the columns left on the current row,
keeping one back. The line's last row therefore stays the line's last row, and
every number the redraw computes from the buffer -- where the cursor goes, how
many rows to climb next time -- is exactly what it was before suggestions
existed. fish lets its suggestions wrap; doing that here would put the wrap
arithmetic, which this editor has already had two defects in, on the keystroke
path for the sake of a decoration.

**It is only offered at the end of the line.** There is no "next" when the cursor
is in the middle, and drawing a guess past the end while editing the middle would
put the two in different places and mean nothing.

Right and End accept it. Both were no-ops at the end of a line, so neither had to
be taken from anything.

## Colour is a prerequisite, not an enhancement

If `\033[90m` does not render, the suggestion appears as ordinary text: the
screen then shows characters that are not in the line, and Enter runs something
shorter than what is visible. That is worse than having no suggestion at all, so
an absent capability turns the feature **off** rather than degrading it. The same
switch covers highlighting, because both ask the same question.

`newTheme` reads `NO_COLOR` (presence, not value, per no-color.org), `TERM=dumb`,
and `NEMOSH_COLOR=always|never` -- spelled with the words `ls --color` already
accepts, so there is one vocabulary rather than two.

## What the colours mean

| role | drawn as | decided by |
| --- | --- | --- |
| a command this shell carries | green | `capability.Known` |
| a command it does not | red | the same, absent |
| an option the command accepts | cyan | `capability.Lookup` |
| an option it does not | yellow | the same, absent |
| the word being edited | underline, over whatever colour applies | the cursor |
| the suggestion | grey | not typed yet |

Unknown is yellow rather than red because it is a guess, not a verdict: an
external program on `PATH` is not in the table, and neither is a function or an
alias this session defined. For the same reason a word belonging to an unknown
command gets no option verdict at all -- inventing one from absence would be
saying something the shell does not know.

Every choice is in `defaultPalette()`, one struct, so changing how a known
command looks is one edit rather than a search.

## How it is tested

The screen model records the SGR parameters in force at **each cell**, not just
the characters. That is what lets a test ask "what exactly is underlined" and
"is the cursor still just past what was typed" -- and the second question is the
one worth having, because text drawn past the cursor occupies columns the buffer
does not know about, which is precisely the defect class that had `$ ` measuring
eleven columns and a wrapped line redrawn over the wrong rows. `\033[4m` is well
formed wherever it appears; only its position can be wrong, and only a model can
see position.

## Not done

- **Persistent history**, which would make the strongest source much stronger.
- **A configurable palette at runtime.** The struct is ready for it; nothing
  reads a setting into it yet.
- **Suggesting an operand**, which needs a bounded, cached view of a directory
  before it can go on the keystroke path.
- **Menu selection**, zsh's `zsh/complist` behaviour, which is a different
  feature again and belongs with completion rather than here.
