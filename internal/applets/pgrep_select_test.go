package applets

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// pgrep and pkill as procps-ng's: -f matches the command line, -a prints it, -u selects by account,
// -n and -o the newest and oldest, -c counts and -d separates. Over a table made up here, with
// details from a map: a pid made up in a test is some real process's (see noDetails).

func pgrepTestTable() pgrepTable {
	at := func(hours float64) time.Time { return psTaken.Add(-time.Duration(hours * float64(time.Hour))) }
	return pgrepTable{
		processes: []proc.Process{
			{PID: 100, PPID: 1, Name: "node.exe", Created: at(3)},
			{PID: 104, PPID: 1, Name: "node.exe", Created: at(1)},
			{PID: 108, PPID: 1, Name: "python.exe", Created: at(2)},
			{PID: 112, PPID: 4, Name: "svchost.exe", Created: at(5)},
			{PID: 116, PPID: 1, Name: "notepad.exe", Created: at(0.5)},
		},
		details: psDetails{
			100: {User: "nemo", CommandLine: "node server.js --port 80"},
			104: {User: "nemo", CommandLine: "node worker.js"},
			108: {User: "nemo", CommandLine: "python3 -m http.server"},
			116: {User: "Administrator", CommandLine: `notepad.exe C:\a.txt`},
		},
	}
}

func TestPgrep_procpsOptions(t *testing.T) {
	for _, test := range []struct {
		args   []string
		want   string
		status int
	}{
		{args: []string{"node"}, want: "100\n104\n"},
		{args: []string{"server"}, status: 1},
		{args: []string{"-f", `server\.js`}, want: "100\n"},
		{args: []string{"-f", "server"}, want: "100\n108\n"},
		{args: []string{"-f", `http\.server`}, want: "108\n"},
		// A command line Windows will not show is matched as its image name, as procps matches a
		// process whose command line is empty.
		{args: []string{"-f", "svchost"}, want: "112\n"},
		{args: []string{"-fx", "node worker.js"}, want: "104\n"},
		{args: []string{"-a", "node"}, want: "100 node server.js --port 80\n104 node worker.js\n"},
		{args: []string{"-a", "svchost"}, want: "112 svchost.exe\n"},
		{args: []string{"-l", "node"}, want: "100 node.exe\n104 node.exe\n"},
		{args: []string{"-u", "nemo", "node"}, want: "100\n104\n"},
		// -u needs no pattern, as procps': `pgrep -u root` is everything root runs.
		{args: []string{"-u", "administrator"}, want: "116\n"},
		{args: []string{"-U", "nemo,administrator", "-f", "notepad"}, want: "116\n"},
		{args: []string{"-u", "nobody"}, status: 1},
		{args: []string{"-n", "node"}, want: "104\n"},
		{args: []string{"-o", "node"}, want: "100\n"},
		{args: []string{"-c", "node"}, want: "2\n"},
		{args: []string{"-c", "zzznosuch"}, want: "0\n", status: 1},
		{args: []string{"-d", ",", "node"}, want: "100,104\n"},
		// svchost's command line is not to be had, so its name is what -f matches.
		{args: []string{"-d,", "-f", "o"}, want: "100,104,108,112,116\n"},
		{args: []string{"-i", "NODE"}, want: "100\n104\n"},
		{args: []string{"-v", "-u", "nemo"}, want: "112\n116\n"},
	} {
		var stdout bytes.Buffer
		err := writePgrep(context.Background(), test.args, pgrepTestTable(), &stdout)
		status := 0
		if err != nil {
			code, ok := StatusCode(err)
			if !ok {
				t.Errorf("pgrep %q: %v", test.args, err)
				continue
			}
			status = code
		}
		if stdout.String() != test.want || status != test.status {
			t.Errorf("pgrep %q: %q, status %d; want %q, %d", test.args, stdout.String(), status, test.want, test.status)
		}
	}
}

// pkill selects as pgrep does, and kills what it selected.
func TestPkill_selectsAsPgrepDoes(t *testing.T) {
	for _, test := range []struct {
		args []string
		want []int
	}{
		{args: []string{"-f", "worker"}, want: []int{104}},
		{args: []string{"-u", "nemo", "-n", "node"}, want: []int{104}},
		{args: []string{"-o", "-f", "node"}, want: []int{100}},
		{args: []string{"-U", "administrator"}, want: []int{116}},
	} {
		matcher, err := parseProcessPattern(context.Background(), "pkill", test.args, pkillLetters)
		if err != nil {
			t.Fatalf("pkill %q: %v", test.args, err)
		}
		var got []int
		for _, match := range matcher.choose(pgrepTestTable()) {
			got = append(got, match.process.PID)
		}
		if len(got) != len(test.want) || (len(got) > 0 && got[0] != test.want[0]) {
			t.Errorf("pkill %q chose %v, want %v", test.args, got, test.want)
		}
	}
}

func TestPgrep_refusesWhatItCannotRead(t *testing.T) {
	for _, test := range []struct {
		args []string
		says string
	}{
		{args: []string{"-u", ""}, says: "user name list syntax error"},
		{args: []string{"-n", "-o", "x"}, says: "-n and -o"},
		{args: []string{"-d"}, says: "option requires an argument -- d"},
	} {
		var stdout bytes.Buffer
		if err := writePgrep(context.Background(), test.args, pgrepTestTable(), &stdout); err == nil || !bytes.Contains([]byte(err.Error()), []byte(test.says)) {
			t.Errorf("pgrep %q: %v, want %q", test.args, err, test.says)
		}
	}
}
