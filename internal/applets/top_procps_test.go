package applets

import (
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// top takes its letters bundled, as getopt does, and procps-ng's -p, -u and -U and its -o FIELD
// sort: `top -bn1`, `top -p 1234`, `top -u nemo`, `top -o %MEM`. Each was refused.
func TestTopArgs_bundlesLettersAndTakesProcpsOptions(t *testing.T) {
	for _, test := range []struct {
		args  []string
		check func(topOptions) bool
	}{
		{[]string{"-bn1"}, func(o topOptions) bool { return o.batch && o.iterations == 1 }},
		{[]string{"-bn", "3"}, func(o topOptions) bool { return o.batch && o.iterations == 3 }},
		{[]string{"-d0.5"}, func(o topOptions) bool { return o.delay == 500*time.Millisecond }},
		{[]string{"-bHt"}, func(o topOptions) bool { return o.batch && o.threads && o.tree }},
		{[]string{"-p", "4,8", "-p12"}, func(o topOptions) bool { return len(o.pids) == 3 && o.pids[2] == 12 }},
		{[]string{"-u", "nemo"}, func(o topOptions) bool { return len(o.users) == 1 && o.users[0] == "nemo" }},
		{[]string{"-U", "nemo,root"}, func(o topOptions) bool { return len(o.users) == 2 }},
		// procps' -o names a field to sort by; the case tells it from this top's column list.
		{[]string{"-o", "%MEM"}, func(o topOptions) bool { return o.sort == "mem" && o.columns == nil && o.direction == 0 }},
		{[]string{"-o", "-PID"}, func(o topOptions) bool { return o.sort == "pid" && o.direction < 0 }},
		{[]string{"-o", "+%CPU"}, func(o topOptions) bool { return o.sort == "cpu" && o.direction > 0 }},
		{[]string{"-o", "RES"}, func(o topOptions) bool { return o.sort == "rss" }},
		{[]string{"-o", "TIME+"}, func(o topOptions) bool { return o.sort == "time" }},
		{[]string{"-o", "pid,cpu"}, func(o topOptions) bool { return len(o.columns) == 2 && o.sort == "cpu" }},
	} {
		options, err := topArgs(test.args)
		if err != nil || !test.check(options) {
			t.Errorf("top %q: %+v, %v", test.args, options, err)
		}
	}
	for _, test := range []struct {
		args []string
		says string
	}{
		{[]string{"-p", "x"}, "invalid process ID list: x"},
		{[]string{"-u", ""}, "user name list syntax error"},
		{[]string{"-o", "VIRT"}, "unknown field: VIRT"},
		{[]string{"-bn"}, "-n requires a value"},
		{[]string{"-bZ"}, "unknown option -- Z"},
	} {
		if _, err := topArgs(test.args); err == nil || err.Error() != test.says {
			t.Errorf("top %q: %v, want %q", test.args, err, test.says)
		}
	}
}

// -p and -u narrow the list as -f does, before anything is drawn: by pid, and by the account the
// process runs as, which a process whose owner Windows will not show has none of.
func TestTopModel_selectsByPidAndAccount(t *testing.T) {
	snapshot := proc.Snapshot{Processes: []proc.Process{
		testProcess(10, 1, "a.exe", 100), testProcess(20, 1, "b.exe", 200), testProcess(30, 1, "c.exe", 300),
	}}
	details := psDetails{10: {User: "nemo"}, 20: {User: "Administrator"}}
	pids := func(rows []topRow) []int {
		var got []int
		for _, row := range rows {
			got = append(got, row.Process.PID)
		}
		return got
	}
	model := newTopModel(mustColumns(t))
	model.setSort("pid")
	model.Pids = map[int]bool{10: true, 30: true}
	if got := pids(model.rows(snapshot, proc.Rates{}, details)); len(got) != 2 || got[0] != 10 || got[1] != 30 {
		t.Errorf("-p 10,30 chose %v", got)
	}
	model.Pids = nil
	model.Users = []string{"ADMINISTRATOR", "nobody"}
	if got := pids(model.rows(snapshot, proc.Rates{}, details)); len(got) != 1 || got[0] != 20 {
		t.Errorf("-u administrator chose %v", got)
	}
}
