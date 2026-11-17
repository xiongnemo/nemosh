package proc

import (
	"os"
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

// A process that is exiting answers the command-line query STATUS_PROCESS_IS_TERMINATING and
// no length. A 32-bit nemosh hears that through WOW64, which takes 8 off the length a 64-bit
// answer needs, for the 64-bit header it does not have -- off 0, so 0xfffffff8, measured.
// commandLine sized a buffer by it and panicked: `top` on windows/386, at whatever process was
// exiting as it sampled. A length is believed only when the query says it is the length needed,
// and only one a UNICODE_STRING could describe.
func TestCommandLineSize_believesOnlyTheLengthTheQueryAsksFor(t *testing.T) {
	for _, test := range []struct {
		name   string
		status error
		needed uint32
		ok     bool
	}{
		{name: "the length asked for", status: windows.STATUS_INFO_LENGTH_MISMATCH, needed: 0x46, ok: true},
		{name: "an exiting process, through WOW64", status: windows.STATUS_PROCESS_IS_TERMINATING, needed: 0xfffffff8},
		{name: "an exiting process", status: windows.STATUS_PROCESS_IS_TERMINATING},
		{name: "a length no command line has", status: windows.STATUS_INFO_LENGTH_MISMATCH, needed: 0xfffffff8},
		{name: "no length at all", status: windows.STATUS_INFO_LENGTH_MISMATCH},
	} {
		t.Run(test.name, func(t *testing.T) {
			size, ok := commandLineSize(test.status, test.needed)
			if ok != test.ok || ok && size != int(test.needed) {
				t.Fatalf("commandLineSize(%v, %#x) = %d, %v; want %v", test.status, test.needed, size, ok, test.ok)
			}
		})
	}
}

// And the process itself: one that has ended, its handle still open, has no command line to
// give, which is an empty answer and not a panic.
func TestCommandLine_ofAnEndedProcessIsEmpty(t *testing.T) {
	child := exec.Command(os.Args[0], "-test.run=^$")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(child.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	if err := child.Wait(); err != nil {
		t.Fatal(err)
	}
	if got := commandLine(handle); got != "" {
		t.Fatalf("an ended process has the command line %q", got)
	}
}
