package applets

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// -xdev evaluates a directory on another volume and does not go into it, as busybox stops at a
// mount point after acting on it. A test cannot mount a volume, so the walk is handed the
// volumes it may stay on: none that the directory is on, and then the one it is on.
func TestFind_xdevStaysOnThePathsVolumes(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "d", "e"), 0o755); err != nil {
		t.Fatal(err)
	}
	record, err := hostStatRecord(dir, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args    []string
		volumes []uint64
		want    string
	}{
		{args: []string{".", "-xdev"}, volumes: []uint64{record.device}, want: ".|./d|./d/e"},
		{args: []string{".", "-xdev"}, volumes: []uint64{record.device + 1}, want: "."},
		{args: []string{".", "-xdev", "-depth"}, volumes: []uint64{record.device}, want: "./d/e|./d|."},
		{args: []string{".", "-xdev", "-depth"}, volumes: []uint64{record.device + 1}, want: "."},
		{args: []string{"."}, volumes: []uint64{record.device + 1}, want: ".|./d|./d/e"},
	} {
		_, expression, err := parseFindArguments(test.args, nil)
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		run := &findRun{stdout: &out, volumes: test.volumes}
		if err := walkFindPath(run, ".", dir, expression); err != nil {
			t.Fatal(err)
		}
		if got := strings.ReplaceAll(strings.TrimSuffix(out.String(), "\n"), "\n", "|"); got != test.want {
			t.Errorf("find %q on volumes %v = %q; want %q", test.args, test.volumes, got, test.want)
		}
	}
}
