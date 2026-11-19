package applets

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// free takes procps-ng's -t, a Total row of memory and swap, -w, buffers and cache apart, and -s
// and -c, the table again every so many seconds or so many times. Each was `unknown option`.
func TestFree_procpsRows(t *testing.T) {
	memory := proc.Memory{TotalPhysical: 16 << 30, AvailablePhysical: 6 << 30, Cached: 3 << 30,
		CommitTotal: 12 << 30, CommitLimit: 20 << 30}
	kib := func(bytes uint64) string { return fmt.Sprint(bytes / 1024) }
	for _, test := range []struct {
		layout freeLayout
		want   string
	}{
		{freeLayout{total: true}, strings.Join([]string{
			fmt.Sprintf("       %12s%12s%12s%12s%12s%12s", "total", "used", "free", "shared", "buff/cache", "available"),
			fmt.Sprintf("Mem:   %12s%12s%12s%12s%12s%12s", kib(16<<30), kib(10<<30), kib(6<<30), "0", kib(3<<30), kib(6<<30)),
			fmt.Sprintf("Swap:  %12s%12s%12s", kib(4<<30), kib(2<<30), kib(2<<30)),
			fmt.Sprintf("Total: %12s%12s%12s", kib(20<<30), kib(12<<30), kib(8<<30)),
		}, "\n") + "\n"},
		{freeLayout{wide: true}, strings.Join([]string{
			fmt.Sprintf("       %12s%12s%12s%12s%12s%12s%12s", "total", "used", "free", "shared", "buffers", "cache", "available"),
			fmt.Sprintf("Mem:   %12s%12s%12s%12s%12s%12s%12s", kib(16<<30), kib(10<<30), kib(6<<30), "0", "0", kib(3<<30), kib(6<<30)),
			fmt.Sprintf("Swap:  %12s%12s%12s", kib(4<<30), kib(2<<30), kib(2<<30)),
		}, "\n") + "\n"},
	} {
		var stdout bytes.Buffer
		if err := writeFreeTable(&stdout, memory, freeScale{unit: 1024}, test.layout); err != nil || stdout.String() != test.want {
			t.Errorf("%+v:\n%s\nwant:\n%s", test.layout, stdout.String(), test.want)
		}
	}
}

// The unit is the first of -b -k -m -g -h given, whatever comes before it: -th is -h with a total.
func TestFree_unitIsTheFirstUnitGiven(t *testing.T) {
	options, _, err := parseAppletLongOptions(context.Background(), []string{"-t", "-h", "-m"}, freeLongOptions, freeFlags, freeValued)
	if err != nil {
		t.Fatal(err)
	}
	if scale := freeScaleFor(options); scale.unit != 0 {
		t.Errorf("free -t -h -m chose a unit of %d, want -h's", scale.unit)
	}
}

// -c prints the table that many times, a blank line between, -s apart.
func TestFree_repeats(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := newFreeApplet().Run(context.Background(), []string{"-c", "2", "-s", "0.01"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("free -c 2 -s 0.01: %v (%s)", err, stderr.String())
	}
	if tables := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n\n"); len(tables) != 2 || !strings.Contains(tables[1], "Mem:") {
		t.Fatalf("free -c 2 printed %q, want two tables a blank line apart", stdout.String())
	}
	for _, args := range [][]string{{"-s", "0"}, {"-s", "x"}, {"-c", "0"}} {
		if err := newFreeApplet().Run(context.Background(), args, strings.NewReader(""), &stdout, &stderr); err == nil {
			t.Errorf("free %q succeeded", args)
		}
	}
}
