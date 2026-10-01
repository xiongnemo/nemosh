package applets

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// free's table is busybox's: a label of seven and values of twelve; -m and -g round to the
// nearest, as busybox's make_human_readable_str does for a unit, and -h scales each value to
// the largest unit that leaves it under 1024, with one decimal. The first option decides, as
// busybox reads only its first argument. Measured against busybox-w32; the header and rows
// were narrower and out of line with each other, the values cut short, and -h ignored.
func TestFreeTable_isBusyboxs(t *testing.T) {
	const gib, mib = 1 << 30, 1 << 20
	total := uint64(3*gib + 700*mib + 600*1024)
	memory := proc.Memory{
		TotalPhysical: total, AvailablePhysical: gib, Cached: 512 * mib,
		CommitLimit: total + 2*gib, CommitTotal: total - gib + gib,
	}
	row := func(label string, values ...string) string {
		line := label
		for _, value := range values {
			line += fmt.Sprintf("%12s", value)
		}
		return line + "\n"
	}
	header := row("       ", "total", "used", "free", "shared", "buff/cache", "available")
	for _, test := range []struct {
		order []byte
		want  string
	}{
		{want: header + row("Mem:   ", "3863128", "2814552", "1048576", "0", "524288", "1048576") +
			row("Swap:  ", "2097152", "1048576", "1048576")},
		{order: []byte("m"), want: header + row("Mem:   ", "3773", "2749", "1024", "0", "512", "1024") +
			row("Swap:  ", "2048", "1024", "1024")},
		{order: []byte("g"), want: header + row("Mem:   ", "4", "3", "1", "0", "1", "1") + row("Swap:  ", "2", "1", "1")},
		{order: []byte("h"), want: header + row("Mem:   ", "3.7G", "2.7G", "1.0G", "0", "512.0M", "1.0G") +
			row("Swap:  ", "2.0G", "1.0G", "1.0G")},
		{order: []byte("mh"), want: header + row("Mem:   ", "3773", "2749", "1024", "0", "512", "1024") +
			row("Swap:  ", "2048", "1024", "1024")},
	} {
		var out bytes.Buffer
		if err := writeFreeTable(&out, memory, freeScaleFor(appletOptions{order: test.order})); err != nil {
			t.Fatal(err)
		}
		if out.String() != test.want {
			t.Errorf("free -%s =\n%s\nwant\n%s", test.order, out.String(), test.want)
		}
	}
}
