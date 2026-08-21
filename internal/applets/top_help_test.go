package applets

import (
	"bytes"
	"strings"
	"testing"
)

// The glossary, which is the legend for people who cannot open the drawn form.
//
// `top -o help`, after htop's `--sort-key help`. It matters more here than on Linux: the drawn form
// needs a real console, and mintty -- Git Bash -- is a named pipe rather than one, so a good many
// terminals on this platform never see F1 at all.
func TestTopGlossary_printsEveryColumnAndItsMeaning(t *testing.T) {
	for _, args := range [][]string{{"-o", "help"}, {"-s", "help"}} {
		options, err := topArgs(args)
		if err != nil {
			t.Fatalf("topArgs %v: %v", args, err)
		}
		if !options.glossary {
			t.Fatalf("%v did not ask for the glossary", args)
		}
	}

	var out bytes.Buffer
	if err := writeTopGlossary(&out); err != nil {
		t.Fatalf("writeTopGlossary: %v", err)
	}
	text := out.String()
	for _, column := range topColumns {
		if !strings.Contains(text, column.Key) || !strings.Contains(text, column.Description) {
			t.Fatalf("the glossary omits %s", column.Key)
		}
	}
	// One set of words for both surfaces, so a grep cannot disagree with the screen. htop's own
	// man page drifted from its source exactly this way: it still documents IO column headers
	// its code renamed several releases ago.
	panel := topHelpPanel(newTopModel(mustColumns(t)))
	for _, column := range topColumns {
		if !strings.Contains(panel, column.Description) {
			t.Fatalf("the panel and the glossary disagree about %s", column.Key)
		}
	}
}

// The meters are explained too, and the one that needs it most is the third.
func TestTopHelpPanel_explainsTheMeters(t *testing.T) {
	panel := topHelpPanel(newTopModel(mustColumns(t)))
	if !strings.Contains(panel, "METERS") {
		t.Fatal("the panel does not explain the header meters")
	}
	// Commit is the term nobody arrives knowing, and calling it swap would be wrong.
	if !strings.Contains(panel, "commit charge") || !strings.Contains(panel, "Not swap") {
		t.Fatal("the panel does not distinguish commit charge from swap")
	}
	// And that a dash is "not measured yet" rather than zero.
	if !strings.Contains(panel, "no rate yet") {
		t.Fatal("the panel does not explain the dashes in the first sample")
	}
}
