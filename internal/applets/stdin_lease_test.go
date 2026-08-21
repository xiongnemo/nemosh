package applets

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

// That an applet can still reach the console through the wrapper the registry puts around its stdin.
//
// Worth a test of its own, because this exact shape has now gone wrong three times and no test
// caught any of them. A wrapper is added for one reason -- cancellation here, synchronisation in
// the writer's case -- forwards the one method it was written for, and silently hides everything
// else the thing underneath could do. The failure is never an error: the caller asks "are you a
// terminal?", hears no, and does something reasonable with the wrong answer. `top` printed one
// plain sample and said standard input was not a terminal, in a console, correctly reporting what
// the wrapper had told it.
//
// The others were descriptorWriter needing TerminalFile so `ls` could grid, and then
// synchronizedWriter needing to forward that, which is why terminalFileOf walks a chain instead of
// checking one hop.

// leaseStub stands in for the runtime's descriptorReader: a reader that can hand out a console.
type leaseStub struct {
	io.Reader
	file   *os.File
	leased int
}

func (s *leaseStub) LeaseStdinFile(context.Context) (*os.File, func(), bool) {
	s.leased++
	return s.file, func() {}, true
}

func TestContextReader_forwardsTheStdinLease(t *testing.T) {
	stub := &leaseStub{Reader: strings.NewReader(""), file: os.Stdin}
	wrapped := contextReader{ctx: context.Background(), reader: stub}

	// When -- what runTopInteractive does with the stdin it is handed
	file, release, ok := leaseTopStdin(context.Background(), wrapped)
	defer release()

	// Then
	if !ok {
		t.Fatal("the lease did not reach through the wrapper, so a TUI applet cannot read keys")
	}
	if file != os.Stdin {
		t.Fatalf("leased %v, want the file the reader underneath offered", file)
	}
	if stub.leased != 1 {
		t.Fatalf("the reader underneath was asked %d times, want once", stub.leased)
	}
}

// A plain reader has no console to offer, and the answer must be no rather than a nil file that
// the caller then uses -- this is the `top | cat` case and the test case.
func TestContextReader_leaseRefusedWhenThereIsNoConsoleUnderneath(t *testing.T) {
	wrapped := contextReader{ctx: context.Background(), reader: strings.NewReader("")}

	// When
	file, release, ok := leaseTopStdin(context.Background(), wrapped)
	defer release()

	// Then
	if ok || file != nil {
		t.Fatalf("leased %v ok=%v from a strings.Reader, want a refusal", file, ok)
	}
}

// An *os.File underneath is the common case -- `nemosh -c top` from a console, where the fd table
// holds the real handle -- and it must not need the file to implement anything.
func TestContextReader_forwardsAPlainFile(t *testing.T) {
	wrapped := contextReader{ctx: context.Background(), reader: os.Stdin}

	// When
	file, release, ok := leaseTopStdin(context.Background(), wrapped)
	defer release()

	// Then
	if !ok || file != os.Stdin {
		t.Fatalf("leased %v ok=%v, want os.Stdin", file, ok)
	}
}

// And that the wrapper is what stdin actually arrives in, so the tests above are about the real
// path rather than about a type nothing uses. Asserted through the registry, which is where the
// wrapping happens.
func TestRegistry_wrapsStdinInSomethingThatCanLease(t *testing.T) {
	var seen io.Reader
	// Through NewRegistry rather than a literal, because NewRegistry is what puts the wrapper on
	// -- a test that built the map itself would assert nothing about the real path.
	registry := NewRegistry(simpleApplet{
		name: "probe",
		runContext: func(_ context.Context, _ []string, stdin io.Reader, _, _ io.Writer) error {
			seen = stdin
			return nil
		},
	})
	applet, ok := registry.Lookup("probe")
	if !ok {
		t.Fatal("the probe applet was not registered")
	}

	// When
	if err := applet.Run(context.Background(), nil, os.Stdin, io.Discard, io.Discard); err != nil {
		t.Fatalf("probe: %v", err)
	}

	// Then -- whatever the registry wraps stdin in, an applet must be able to ask it for a console
	if _, isLeaser := seen.(stdinFile); !isLeaser {
		t.Fatalf("applets receive %T, which cannot be asked for the console", seen)
	}
}
