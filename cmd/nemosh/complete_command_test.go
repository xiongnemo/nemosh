package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// A command on PATH must complete like any other command.
//
// It used to be left out because walking PATH on every Tab would be felt --
// dozens of directories and thousands of files. That is still true and no longer
// applies: the index the suggestion engine already builds, once per PATH and in
// the background, answers this as a map lookup. So `gi` finishes to `git`, which
// it could not before.
func TestCompleteCommand_reachesPath(t *testing.T) {
	// Given
	commands := newShellCommands(settledIndex(t, seedPathDirectory(t, "wsl.exe", "where.exe", "git.exe")))
	commands.set([]string{"ll"})
	candidates := commands.candidates()

	for _, test := range []struct {
		name   string
		prefix string
		want   []string
	}{
		{name: "a program only PATH knows", prefix: "gi", want: []string{"git"}},
		{name: "programs and applets together", prefix: "w", want: []string{"wait", "wc", "wget", "where", "which", "whoami", "whois", "winpath", "wsl"}},
		{name: "an alias this session defined", prefix: "ll", want: []string{"ll"}},
		{name: "nothing at all", prefix: "zzzznosuch", want: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := completeCommand(test.prefix, candidates)

			// Then
			if !slices.Equal(got, test.want) {
				t.Fatalf("completeCommand(%q) = %v, want %v", test.prefix, got, test.want)
			}
		})
	}
}

// The same name reached two ways is one candidate. `wc` is an applet and could
// equally be a wc.exe on PATH, and offering it twice would make the list read
// like there are two of them.
func TestCompleteCommand_offersEachNameOnce(t *testing.T) {
	// Given: a PATH entry that collides with an applet, and an alias that
	// collides with a builtin
	commands := newShellCommands(settledIndex(t, seedPathDirectory(t, "wc.exe")))
	commands.set([]string{"cd"})

	// When
	got := completeCommand("wc", commands.candidates())

	// Then
	if !slices.Equal(got, []string{"wc"}) {
		t.Fatalf("completeCommand(wc) = %v, want one entry", got)
	}
	if got := completeCommand("cd", commands.candidates()); !slices.Equal(got, []string{"cd"}) {
		t.Fatalf("completeCommand(cd) = %v, want one entry", got)
	}
}

// Tab after typing one letter takes the user as far as the choice is, and shows
// the choice. This is the case that prompted it: `w` suggests `wc`, and Tab
// should say what else `w` could be.
func TestCompleteCommand_listsTheChoiceAfterOneLetter(t *testing.T) {
	// Given
	screen, editor := newStyledEditor(t, 80, "", nil)
	programs := seedPathDirectory(t, "wsl.exe", "where.exe")
	editor.commands.path.refresh(programs)
	waitForPathBuiltFrom(t, editor.commands.path, programs)

	// When: `w`, then Tab
	editor.buffer.insert('w')
	editor.complete("$ ")

	// Then: every `w` command is on screen, from both sources.
	painted := screenText(screen)
	for _, want := range []string{"wait", "wc", "winpath", "wsl", "where"} {
		if !strings.Contains(painted, want) {
			t.Fatalf("the listing does not mention %q:\n%s", want, painted)
		}
	}
	// And the line is unchanged, because `w` is already all they share.
	if got := editor.buffer.String(); got != "w" {
		t.Fatalf("buffer = %q, want it left at w -- that is the whole shared prefix", got)
	}
}

// A listing that would fill the screen asks before printing.
//
// A bare Tab has some two thousand answers on an ordinary Windows machine now
// that command completion reaches PATH. Printing that unasked scrolls the
// session away; refusing outright takes the decision from someone who may well
// want to look. bash asks, and so does this.
func TestCompleteCommand_asksBeforeFillingTheScreen(t *testing.T) {
	// Enough names to need more rows than the budget allows.
	const plenty = 4000
	many := make([]string, 0, plenty)
	for index := range plenty {
		many = append(many, fmt.Sprintf("zz%04d", index))
	}

	for _, test := range []struct {
		name   string
		answer string
		listed bool
	}{
		{name: "yes prints them", answer: "y", listed: true},
		{name: "anything else does not", answer: "n", listed: false},
		{name: "no answer at all does not", answer: "", listed: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			screen, editor := newStyledEditor(t, 80, test.answer, nil)
			editor.commands.set(many)
			waitForPathIndex(t, editor.commands.path)

			// When
			for _, r := range "zz" {
				editor.buffer.insert(r)
			}
			editor.complete("$ ")

			// Then
			painted := screenText(screen)
			if !strings.Contains(painted, "Display all 4000 possibilities?") {
				t.Fatalf("want the question, got:\n%s", painted)
			}
			if listed := strings.Contains(painted, many[0]); listed != test.listed {
				t.Fatalf("listed = %v, want %v; screen:\n%s", listed, test.listed, firstRows(painted, 4))
			}
		})
	}
}

func firstRows(text string, count int) string {
	rows := strings.SplitN(text, "\n", count+1)
	if len(rows) > count {
		rows = rows[:count]
	}
	return strings.Join(rows, "\n")
}

// Below the threshold the names are still what is printed.
func TestCompleteCommand_stillListsASmallChoice(t *testing.T) {
	// Given
	screen, editor := newStyledEditor(t, 80, "", nil)
	editor.commands.set([]string{"zza", "zzb"})
	waitForPathIndex(t, editor.commands.path)

	// When
	for _, r := range "zz" {
		editor.buffer.insert(r)
	}
	editor.complete("$ ")

	// Then
	painted := screenText(screen)
	if !strings.Contains(painted, "zza") || !strings.Contains(painted, "zzb") {
		t.Fatalf("want both names listed, got:\n%s", painted)
	}
}

// A command name is completed from a fresh command position too, not only at the
// start of the line.
func TestCompleteCommand_worksAfterAPipe(t *testing.T) {
	// Given
	_, editor := newStyledEditor(t, 80, "", nil)
	waitForPathIndex(t, editor.commands.path)

	// When
	for _, r := range "echo hi | ec" {
		editor.buffer.insert(r)
	}
	editor.complete("$ ")

	// Then
	if got := editor.buffer.String(); got != "echo hi | echo " {
		t.Fatalf("buffer = %q, want the command after the pipe completed", got)
	}
}

func screenText(screen *screenModel) string {
	var painted strings.Builder
	for row := range screen.rowCount() {
		painted.WriteString(screen.text(row))
		painted.WriteString("\n")
	}
	return painted.String()
}
