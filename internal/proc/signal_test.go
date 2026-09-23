package proc

import (
	"errors"
	"testing"
)

// One table for kill, pkill and killall: what it accepts, what it refuses, and why.
func TestParseSignal(t *testing.T) {
	for _, test := range []struct {
		spec string
		want int
		err  error
	}{
		{spec: "9", want: 9},
		{spec: "KILL", want: 9},
		{spec: "sigkill", want: 9},
		{spec: "TERM", want: 15},
		// Zero is a question, not a signal, and it is accepted so it can be asked.
		{spec: "0", want: 0},
		// The stop-and-continue family is refused by name and by number, never delivered:
		// delivered here it would end the process it was meant to pause.
		{spec: "STOP", err: ErrCannotSuspend},
		{spec: "SIGCONT", err: ErrCannotSuspend},
		{spec: "tstp", err: ErrCannotSuspend},
		{spec: "19", err: ErrCannotSuspend},
		{spec: "18", err: ErrCannotSuspend},
		{spec: "BOGUS", err: ErrUnknownSignal},
		{spec: "-3", err: ErrUnknownSignal},
	} {
		t.Run(test.spec, func(t *testing.T) {
			got, err := ParseSignal(test.spec)
			if test.err != nil {
				if !errors.Is(err, test.err) {
					t.Fatalf("ParseSignal(%q) = %d, %v; want %v", test.spec, got, err, test.err)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("ParseSignal(%q) = %d, %v; want %d", test.spec, got, err, test.want)
			}
		})
	}
}

// The list is in number order and names only what the table would accept.
func TestSignals_listsOnlyWhatCanBeSent(t *testing.T) {
	previous := -1
	for _, signal := range Signals() {
		if signal.Number <= previous {
			t.Fatalf("%s (%d) is out of order", signal.Name, signal.Number)
		}
		previous = signal.Number
		if number, err := ParseSignal(signal.Name); err != nil || number != signal.Number {
			t.Fatalf("%s is listed but ParseSignal gives %d, %v", signal.Name, number, err)
		}
	}
	if previous < 0 {
		t.Fatal("no signals listed")
	}
}
