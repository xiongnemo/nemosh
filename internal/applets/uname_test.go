package applets

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestUnameApplet_returnsName_whenConstructed(t *testing.T) {
	// Given
	applet := newUnameApplet()

	// When
	got := applet.Name()

	// Then
	if got != "uname" {
		t.Fatalf("expected uname applet name, got %q", got)
	}
}

func TestUnameApplet_printsSysname_whenRunWithoutOptions(t *testing.T) {
	// Given
	applet := newDeterministicUnameApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), nil, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected uname default to succeed, got %v", err)
	}
	if got := stdout.String(); got != "Windows_NT\n" {
		t.Fatalf("expected default sysname output, got %q", got)
	}
}

func TestUnameApplet_printsSelectedFieldsInBusyBoxOrder_whenRunWithClusteredOptions(t *testing.T) {
	// Given
	applet := newDeterministicUnameApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-mns"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected uname clustered options to succeed, got %v", err)
	}
	if got := stdout.String(); got != "Windows_NT nemosh-host x86_64\n" {
		t.Fatalf("expected BusyBox field order, got %q", got)
	}
}

func TestUnameApplet_printsAllKnownFields_whenRunWithAllOption(t *testing.T) {
	// Given
	applet := newDeterministicUnameApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-a"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected uname -a to succeed, got %v", err)
	}
	if got := stdout.String(); got != "Windows_NT nemosh-host 10.0 22631 x86_64 MS/Windows\n" {
		t.Fatalf("expected uname -a output, got %q", got)
	}
}

func TestUnameApplet_printsUnknown_whenRunWithExplicitProcessorOrPlatformOption(t *testing.T) {
	// Given
	applet := newDeterministicUnameApplet()
	tests := []struct {
		name string
		args []string
	}{
		{name: "processor", args: []string{"-p"}},
		{name: "hardware platform", args: []string{"-i"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer

			// When
			err := applet.Run(context.Background(), tt.args, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

			// Then
			if err != nil {
				t.Fatalf("expected uname %v to succeed, got %v", tt.args, err)
			}
			if got := stdout.String(); got != "unknown\n" {
				t.Fatalf("expected explicit unknown output, got %q", got)
			}
		})
	}
}

func TestUnameApplet_acceptsBusyBoxLongOptions_whenRunWithLongOption(t *testing.T) {
	// Given
	applet := newDeterministicUnameApplet()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "all", args: []string{"--all"}, want: "Windows_NT nemosh-host 10.0 22631 x86_64 MS/Windows\n"},
		{name: "kernel name", args: []string{"--kernel-name"}, want: "Windows_NT\n"},
		{name: "nodename", args: []string{"--nodename"}, want: "nemosh-host\n"},
		{name: "kernel release", args: []string{"--kernel-release"}, want: "10.0\n"},
		{name: "release alias", args: []string{"--release"}, want: "10.0\n"},
		{name: "kernel version", args: []string{"--kernel-version"}, want: "22631\n"},
		{name: "machine", args: []string{"--machine"}, want: "x86_64\n"},
		{name: "processor", args: []string{"--processor"}, want: "unknown\n"},
		{name: "hardware platform", args: []string{"--hardware-platform"}, want: "unknown\n"},
		{name: "operating system", args: []string{"--operating-system"}, want: "MS/Windows\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer

			// When
			err := applet.Run(context.Background(), tt.args, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

			// Then
			if err != nil {
				t.Fatalf("expected uname %v to succeed, got %v", tt.args, err)
			}
			if got := stdout.String(); got != tt.want {
				t.Fatalf("expected long option output %q, got %q", tt.want, got)
			}
		})
	}
}

func TestUnameApplet_returnsUnsupportedOptionError_whenRunWithUnknownLongOption(t *testing.T) {
	// Given
	applet := newDeterministicUnameApplet()

	// When
	err := applet.Run(context.Background(), []string{"--unknown"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if err == nil || err.Error() != "unsupported uname option: --unknown" {
		t.Fatalf("expected unsupported long option error, got %v", err)
	}
}

func TestUnameApplet_returnsUnsupportedOptionError_whenRunWithUnknownOption(t *testing.T) {
	// Given
	applet := newDeterministicUnameApplet()

	// When
	err := applet.Run(context.Background(), []string{"-z"}, &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{})

	// Then
	if err == nil || err.Error() != "unsupported uname option: -z" {
		t.Fatalf("expected unsupported option error, got %v", err)
	}
}

func TestUnameApplet_returnsErrExitFalse_whenRunWithOperand(t *testing.T) {
	// Given
	applet := newDeterministicUnameApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"extra"}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if !errors.Is(err, ErrExitFalse) {
		t.Fatalf("expected unexpected operand to return ErrExitFalse, got %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected no output for unexpected operand, got %q", got)
	}
}

func newDeterministicUnameApplet() Applet {
	return unameApplet{info: func() unameInfo {
		return unameInfo{
			sysname:   "Windows_NT",
			nodename:  "nemosh-host",
			release:   "10.0",
			version:   "22631",
			machine:   "x86_64",
			processor: "unknown",
			platform:  "unknown",
			os:        "MS/Windows",
		}
	}}
}
