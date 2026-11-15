package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// A history file in bash's format, as bash writes it under HISTTIMEFORMAT: a # and the time
// before each entry, and the lines up to the next one the entry, so a command of several lines
// reads back whole. A # line was read as a command, so a bash history file loaded its
// timestamps as things that had been run. A file with none is one entry a line, as before.
func TestParseHistoryFile_readsBashsTimestamps(t *testing.T) {
	records := runtime.ParseHistoryFile("#1700000000\nfor i in 1\ndo echo $i\ndone\n#1700000060\nls\n")
	if len(records) != 2 || records[0].Line != "for i in 1\ndo echo $i\ndone" || records[1].Line != "ls" {
		t.Fatalf("records = %+v", records)
	}
	if !records[0].When.Equal(time.Unix(1700000000, 0)) || !records[1].When.Equal(time.Unix(1700000060, 0)) {
		t.Fatalf("times = %v, %v", records[0].When, records[1].When)
	}
	if plain := runtime.ParseHistoryFile("ls\n\n#not a time\necho\n"); len(plain) != 3 || plain[1].Line != "#not a time" || !plain[0].When.IsZero() {
		t.Fatalf("plain records = %+v", plain)
	}
}

// HISTTIMEFORMAT puts each entry's time before it in `history`, through strftime, and has the
// history file written with the times, as bash writes it -- and then a command of several
// lines is written whole, which without the times would read back as several commands.
func TestRuntime_historyKeepsTimesUnderHisttimeformat(t *testing.T) {
	// Given
	var out bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &out, Stderr: &out})
	file := filepath.ToSlash(filepath.Join(t.TempDir(), "history"))
	rt.RunScript(context.Background(), "HISTTIMEFORMAT='%Y|'\nHISTFILE='"+file+"'\n")
	rt.RecordInteractiveLine("echo hi", false)
	rt.RecordInteractiveLine("for i in 1\ndo echo $i\ndone", false)

	// When
	rt.RunHistoryBuiltin(nil)
	rt.RunHistoryBuiltin([]string{"-w"})

	// Then: the listing has the year before each command
	if !regexp.MustCompile(`^ *1  \d{4}\|echo hi\n *2  \d{4}\|for i in 1\ndo echo \$i\ndone\n$`).MatchString(out.String()) {
		t.Fatalf("history = %q", out.String())
	}
	// And the file has a time before each, the second whole
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^#\d+\necho hi\n#\d+\nfor i in 1\ndo echo \$i\ndone\n$`).Match(data) {
		t.Fatalf("file = %q", data)
	}
	// And another shell reads it back as the same two commands
	fresh := runtime.New(applets.DefaultRegistry, runtime.Streams{})
	fresh.RunScript(context.Background(), "HISTFILE='"+file+"'\nhistory -r\n")
	if got := fresh.HistoryEntries(); !slices.Equal(got, []string{"echo hi", "for i in 1\ndo echo $i\ndone"}) {
		t.Fatalf("read back %q, want the two commands and no times", got)
	}
}
