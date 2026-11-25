package sudo

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestRequest_readsBackAsItWasWritten(t *testing.T) {
	// Given
	want := Request{Shell: true, Handles: [3]uint64{0x1c, 0, 0x2a8}, Dir: `C:\Users\me`,
		Args: []string{"id", "-u", ""}, Env: []string{"PATH=C:\\bin", "SSH_CONNECTION=::1 1 ::1 22"}}
	var wire bytes.Buffer

	// When
	if err := WriteRequest(&wire, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRequest(&wire)

	// Then
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("read %+v, wrote %+v", got, want)
	}
}

func TestReply_readsBackAsItWasWritten(t *testing.T) {
	for _, want := range []Reply{{Status: 0}, {Status: 127, Message: "nosuch: not found"}, {Status: -1073741510}} {
		var wire bytes.Buffer
		if err := WriteReply(&wire, want); err != nil {
			t.Fatal(err)
		}
		if got, err := ReadReply(&wire); err != nil || got != want {
			t.Fatalf("read %+v, %v; wrote %+v", got, err, want)
		}
	}
}

// What arrives on the pipe comes from any process in the session, so it is read with limits: a
// length that runs past them is refused, not allocated.
func TestRequest_refusesWhatRunsPastItsLimits(t *testing.T) {
	for _, wire := range []string{
		"",
		"XXXX",
		requestMagic + "\x00\x00\x00\x00" + strings.Repeat("\x00", 24) + "\xff\xff\xff\x7f",
	} {
		if _, err := ReadRequest(strings.NewReader(wire)); err == nil {
			t.Errorf("read %q without complaint", wire)
		}
	}
}
