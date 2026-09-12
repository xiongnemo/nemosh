package applets

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// cal, getopt and ipcalc.
//
// cal is checked byte for byte against busybox-w32 where it is installed, because the
// layout *is* the specification: a calendar that is right about the dates and wrong about
// the column widths is still wrong. The literal cases below are what holds elsewhere.

func TestCalMonth(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "a month", args: []string{"3", "2026"},
			want: "     March 2026\n" +
				"Su Mo Tu We Th Fr Sa\n" +
				" 1  2  3  4  5  6  7\n" +
				" 8  9 10 11 12 13 14\n" +
				"15 16 17 18 19 20 21\n" +
				"22 23 24 25 26 27 28\n" +
				"29 30 31\n" +
				"                     \n",
		},
		{
			// The month that is short eleven days, which every cal prints this way.
			name: "September 1752", args: []string{"9", "1752"},
			want: "   September 1752\n" +
				"Su Mo Tu We Th Fr Sa\n" +
				"       1  2 14 15 16\n" +
				"17 18 19 20 21 22 23\n" +
				"24 25 26 27 28 29 30\n" +
				"                     \n" +
				"                     \n" +
				"                     \n",
		},
		{
			name: "a week starting on Monday", args: []string{"-m", "3", "2026"},
			want: "     March 2026\n" +
				"Mo Tu We Th Fr Sa Su\n" +
				"                   1\n" +
				" 2  3  4  5  6  7  8\n" +
				" 9 10 11 12 13 14 15\n" +
				"16 17 18 19 20 21 22\n" +
				"23 24 25 26 27 28 29\n" +
				"30 31\n",
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, stderr, status := runApplet(t, "cal", testcase.args, "")
			if got != testcase.want || stderr != "" || status != 0 {
				t.Fatalf("cal %v\n got %q\nwant %q\nstderr %q status %d",
					testcase.args, got, testcase.want, stderr, status)
			}
			checkCalAgainstBusybox(t, testcase.args, got)
		})
	}
}

// TestCalAgainstBusybox sweeps a range of months and the year view.
//
// Not a list of literals: the point is that the layout agrees everywhere, and forty
// expected calendars written out by hand would be forty chances to write down what this
// code does rather than what is right.
func TestCalAgainstBusybox(t *testing.T) {
	t.Parallel()
	if !busyboxIsTheReference() {
		t.Skip("busybox-w32 is not installed")
	}
	var cases [][]string
	for _, year := range []string{"1752", "1900", "2000", "2026", "2027"} {
		for month := 1; month <= 12; month++ {
			cases = append(cases, []string{strconv.Itoa(month), year})
		}
		cases = append(cases, []string{year}, []string{"-m", year})
	}
	for _, args := range cases {
		got, _, status := runApplet(t, "cal", args, "")
		if status != 0 {
			t.Fatalf("cal %v exited %d", args, status)
		}
		checkCalAgainstBusybox(t, args, got)
	}
	t.Logf("compared %d calendars against busybox", len(cases))
}

func checkCalAgainstBusybox(t *testing.T, args []string, got string) {
	t.Helper()
	if !busyboxIsTheReference() {
		return
	}
	path, err := exec.LookPath("busybox")
	if err != nil {
		return
	}
	out, err := exec.Command(path, append([]string{"cal"}, args...)...).Output()
	if err != nil {
		t.Fatalf("could not run busybox cal %v: %v", args, err)
	}
	// busybox writes CRLF on Windows; the comparison is about the layout, not the
	// line ending this shell chose.
	want := strings.ReplaceAll(string(out), "\r\n", "\n")
	if got != want {
		t.Fatalf("cal %v disagrees with busybox\n got %q\nwant %q", args, got, want)
	}
}

func TestCalRefusals(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"13", "2026"}, {"0", "2026"}, {"x"}, {"3", "0"}, {"1", "2", "3"}, {"-j"}} {
		if _, _, status := runApplet(t, "cal", args, ""); status == 0 {
			t.Fatalf("cal %v was accepted", args)
		}
	}
}

func TestGetopt(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name   string
		args   []string
		want   string
		status int
	}{
		{name: "short options and an operand", args: []string{"-o", "ab:", "--", "-a", "-b", "v", "x"}, want: " -a -b 'v' -- 'x'\n"},
		{name: "a bundle is split", args: []string{"-o", "abc", "--", "-abc"}, want: " -a -b -c --\n"},
		{name: "a joined value", args: []string{"-o", "ab:", "--", "-bvalue"}, want: " -b 'value' --\n"},
		{name: "long options", args: []string{"-o", "a", "-l", "alpha,beta:", "--", "--alpha", "--beta", "v"}, want: " --alpha --beta 'v' --\n"},
		{name: "a long option joined with equals", args: []string{"-o", "a", "-l", "beta:", "--", "--beta=v"}, want: " --beta 'v' --\n"},
		{name: "unquoted", args: []string{"-u", "-o", "ab:", "--", "-a", "-b", "v", "x"}, want: " -a -b v -- x\n"},
		{
			// The old form is unquoted too, because a script written for the original
			// getopt splits the output on blanks itself.
			name: "the old form takes the optstring as an operand", args: []string{"ab:", "-a", "-b", "v"}, want: " -a -b v --\n",
		},
		{name: "operands are permuted to the end", args: []string{"-o", "a", "--", "x", "-a", "y"}, want: " -a -- 'x' 'y'\n"},
		{name: "a literal -- ends the options", args: []string{"-o", "a", "--", "-a", "--", "-a"}, want: " -a -- '-a'\n"},
		{name: "a quote in an operand survives", args: []string{"-o", "a", "--", "it's"}, want: ` -- 'it'\''s'` + "\n", status: 0},
		{name: "an unknown option still prints what it understood", args: []string{"-o", "a", "--", "-z"}, want: " --\n", status: 1},
		{name: "-Q prints nothing", args: []string{"-Q", "-o", "a", "--", "-a"}, want: ""},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, _, status := runApplet(t, "getopt", testcase.args, "")
			if got != testcase.want || status != testcase.status {
				t.Fatalf("getopt %v\n got %q status %d\nwant %q status %d",
					testcase.args, got, status, testcase.want, testcase.status)
			}
		})
	}
	// -T is how a script asks whether this is the enhanced getopt.
	if out, _, status := runApplet(t, "getopt", []string{"-T"}, ""); status != 4 || out != "" {
		t.Fatalf("getopt -T gave %q status %d, want empty and 4", out, status)
	}
	if _, _, status := runApplet(t, "getopt", []string{"-s", "csh", "-o", "a", "--", "-a"}, ""); status == 0 {
		t.Fatal("getopt -s csh was accepted")
	}
}

func TestIpcalc(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name string
		args []string
		want string
	}{
		{name: "everything from a prefix", args: []string{"-n", "-b", "-m", "-p", "192.168.1.10/24"},
			want: "NETMASK=255.255.255.0\nBROADCAST=192.168.1.255\nNETWORK=192.168.1.0\nPREFIX=24\n"},
		{
			// The old class rule, which is what makes a bare address answer anything.
			name: "a bare class A address", args: []string{"-m", "-p", "10.1.2.3"}, want: "NETMASK=255.0.0.0\nPREFIX=8\n",
		},
		{name: "a bare class B address", args: []string{"-m", "-p", "172.16.0.1"}, want: "NETMASK=255.255.0.0\nPREFIX=16\n"},
		{name: "a bare class C address", args: []string{"-m", "-p", "192.168.1.1"}, want: "NETMASK=255.255.255.0\nPREFIX=24\n"},
		{name: "a netmask operand", args: []string{"-n", "-b", "10.0.0.5", "255.255.255.0"},
			want: "BROADCAST=10.0.0.255\nNETWORK=10.0.0.0\n"},
		{name: "a thirty-one", args: []string{"-b", "-n", "10.0.0.1/31"}, want: "BROADCAST=10.0.0.1\nNETWORK=10.0.0.0\n"},
		{name: "a thirty-two", args: []string{"-b", "-n", "10.0.0.1/32"}, want: "BROADCAST=10.0.0.1\nNETWORK=10.0.0.1\n"},
		{
			// A mask with holes is used as given: the operation is a bitwise and, which
			// is well defined whether or not the mask is contiguous.
			name: "a non-contiguous netmask", args: []string{"-n", "1.2.3.4", "255.0.255.0"}, want: "NETWORK=1.0.3.0\n",
		},
		{name: "the output order is fixed", args: []string{"-p", "-n", "-m", "192.168.1.10/24"},
			want: "NETMASK=255.255.255.0\nNETWORK=192.168.1.0\nPREFIX=24\n"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, stderr, status := runApplet(t, "ipcalc", testcase.args, "")
			if got != testcase.want || stderr != "" || status != 0 {
				t.Fatalf("ipcalc %v\n got %q\nwant %q\nstderr %q status %d",
					testcase.args, got, testcase.want, stderr, status)
			}
		})
	}
	for _, args := range [][]string{{"-n", "999.1.1.1"}, {"-p", "1.2.3.4/33"}, {"-n"}, {"192.168.1.1"}} {
		if _, _, status := runApplet(t, "ipcalc", args, ""); status == 0 {
			t.Fatalf("ipcalc %v was accepted", args)
		}
	}
	// -s asks for the status without the diagnostic.
	if _, stderr, status := runApplet(t, "ipcalc", []string{"-s", "-n", "999.1.1.1"}, ""); status == 0 || stderr != "" {
		t.Fatalf("ipcalc -s: stderr %q status %d", stderr, status)
	}
}
