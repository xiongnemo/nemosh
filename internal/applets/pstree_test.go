package applets

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// pstree is psmisc's (src/pstree.c dump_tree): a child's line indented to its parent's name, the
// first child on the parent's line after ─┬─ or ───, identical siblings folded into N*[...], threads
// as {name}, -p's pids, -a's arguments one process a line, -A's and -U's lines, a PID's tree alone.
// There was no pstree.
func TestPstree_drawsAsPsmiscDoes(t *testing.T) {
	start := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	process := func(pid, ppid int, name string, threads int) proc.Process {
		return proc.Process{PID: pid, PPID: ppid, Name: name, Threads: threads, Created: start.Add(time.Duration(pid) * time.Second)}
	}
	source := pstreeSource{
		processes: func() ([]proc.Process, error) {
			return []proc.Process{
				process(1, 0, "systemd", 1), process(10, 1, "sshd", 1), process(11, 10, "sshd", 1), process(12, 11, "bash", 1),
				process(13, 12, "pstree", 1), process(14, 12, "sleep", 1), process(20, 1, "agetty", 1), process(21, 1, "agetty", 1),
				process(30, 1, "ModemManager", 3), process(40, 1, "cron", 1),
			}, nil
		},
		arguments: func(process proc.Process) string {
			return map[int]string{10: "-D", 11: "-D", 14: "100"}[process.PID]
		},
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-U"}, "systemd─┬─ModemManager───2*[{ModemManager}]\n" +
			"        ├─2*[agetty]\n" +
			"        ├─cron\n" +
			"        └─sshd───sshd───bash─┬─pstree\n" +
			"                             └─sleep\n"},
		{nil, "systemd-+-ModemManager---2*[{ModemManager}]\n" +
			"        |-2*[agetty]\n" +
			"        |-cron\n" +
			"        `-sshd---sshd---bash-+-pstree\n" +
			"                             `-sleep\n"},
		{[]string{"-p"}, "systemd(1)-+-ModemManager(30)---2*[{ModemManager}]\n" +
			"           |-agetty(20)\n" +
			"           |-agetty(21)\n" +
			"           |-cron(40)\n" +
			"           `-sshd(10)---sshd(11)---bash(12)-+-pstree(13)\n" +
			"                                            `-sleep(14)\n"},
		{[]string{"-T", "-c"}, "systemd-+-ModemManager\n" +
			"        |-agetty\n" +
			"        |-agetty\n" +
			"        |-cron\n" +
			"        `-sshd---sshd---bash-+-pstree\n" +
			"                             `-sleep\n"},
		{[]string{"12"}, "bash-+-pstree\n     `-sleep\n"},
		{[]string{"-a"}, "systemd\n" +
			"  |-ModemManager\n" +
			"  |   `-2*[{ModemManager}]\n" +
			"  |-agetty\n" +
			"  |-agetty\n" +
			"  |-cron\n" +
			"  `-sshd -D\n" +
			"      `-sshd -D\n" +
			"          `-bash\n" +
			"              |-pstree\n" +
			"              `-sleep 100\n"},
		{[]string{"-n", "-T", "1"}, "systemd-+-sshd---sshd---bash-+-pstree\n" +
			"        |                    `-sleep\n" +
			"        |-2*[agetty]\n" +
			"        |-ModemManager\n" +
			"        `-cron\n"},
	} {
		var stdout bytes.Buffer
		if err := runPstree(context.Background(), test.args, source, &stdout); err != nil || stdout.String() != test.want {
			t.Errorf("pstree %q = %v\n%s\nwant\n%s", test.args, err, stdout.String(), test.want)
		}
	}
	var stdout bytes.Buffer
	if err := runPstree(context.Background(), []string{"99999"}, source, &stdout); err == nil || stdout.Len() != 0 {
		t.Errorf("pstree of no such process = %q, %v", stdout.String(), err)
	}
	if err := runPstree(context.Background(), []string{"root"}, source, &stdout); err == nil || !strings.Contains(err.Error(), "ps -u") {
		t.Errorf("pstree USER = %v; want it refused with a pointer to ps -u", err)
	}
}

// On this machine pstree draws something, the System process among it.
func TestPstree_drawsThisMachine(t *testing.T) {
	stdout, stderr, status := runApplet(t, "pstree", []string{"-l"}, "")
	if status != 0 || !strings.Contains(stdout, "System") {
		t.Errorf("pstree = %q, %s, status %d", stdout, stderr, status)
	}
	t.Logf("pstree -l:\n%s", stdout)
}
