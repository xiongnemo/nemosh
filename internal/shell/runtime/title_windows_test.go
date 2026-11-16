package runtime

import (
	"bytes"
	"testing"
)

// The console's own title, set and read back through the builtin, and put back as it was.
func TestTitle_setsAndReadsThisConsolesTitle(t *testing.T) {
	saved, err := hostConsoleTitle{}.title()
	if err != nil {
		t.Skipf("no console title to ask: %v", err)
	}
	defer func() { _ = hostConsoleTitle{}.setTitle(saved) }()
	var stdout, stderr bytes.Buffer

	// When
	setStatus := runTitle([]string{"nemosh 标题 test"}, hostConsoleTitle{}, &stdout, &stderr, "")
	readStatus := runTitle(nil, hostConsoleTitle{}, &stdout, &stderr, "")

	// Then
	if setStatus != 0 || readStatus != 0 || stdout.String() != "nemosh 标题 test\n" {
		t.Fatalf("set %d, read %d, printed %q, %q", setStatus, readStatus, stdout.String(), stderr.String())
	}
}
