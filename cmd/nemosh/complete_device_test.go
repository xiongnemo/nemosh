package main

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// `/dev/<TAB>` offers the devices.
//
// Stage 2 of docs/design/device-filesystem.md. The names come from the runtime rather than from a
// list here, because whether `/dev` exists at all is a path-model setting -- a completion that
// carried its own list could offer a device the shell would then refuse to open.

func deviceNamesFromRuntime(t *testing.T) []string {
	t.Helper()
	names := completionDevices(runtime.New(applets.DefaultRegistry, runtime.Streams{}))
	if len(names) == 0 {
		t.Fatal("the runtime reported no devices, so there is nothing to complete")
	}
	return names
}

func TestCompleteDevicePath_offersTheDevices(t *testing.T) {
	where := completionPaths{workingDirectory: t.TempDir(), devices: deviceNamesFromRuntime(t)}

	all := completePathsIn(where, "/dev/", false)
	for _, want := range []string{"/dev/null", "/dev/zero", "/dev/clipboard", "/dev/stdout"} {
		if !slices.Contains(all, want) {
			t.Fatalf("/dev/ offered %v, missing %q", all, want)
		}
	}

	// A stem narrows it, so this is matching rather than listing.
	narrowed := completePathsIn(where, "/dev/ra", false)
	if !slices.Equal(narrowed, []string{"/dev/random"}) {
		t.Fatalf("/dev/ra offered %v, want just /dev/random", narrowed)
	}
}

// A device is not a directory, so a command that takes only directories is offered nothing rather
// than something it cannot use.
func TestCompleteDevicePath_directoriesOnlyOffersNoDevice(t *testing.T) {
	where := completionPaths{workingDirectory: t.TempDir(), devices: deviceNamesFromRuntime(t)}

	if got := completePathsIn(where, "/dev/", true); len(got) != 0 {
		t.Fatalf("cd /dev/ offered %v, want nothing: a device is not a directory", got)
	}
}

// Nothing lives under a device, so a word reaching further gets no answer rather than the whole
// list again.
func TestCompleteDevicePath_nothingUnderADevice(t *testing.T) {
	where := completionPaths{workingDirectory: t.TempDir(), devices: deviceNamesFromRuntime(t)}

	if got := completePathsIn(where, "/dev/null/x", false); len(got) != 0 {
		t.Fatalf("/dev/null/x offered %v, want nothing", got)
	}
}

// With the path model's /dev switched off, no device is offered -- the completion follows the
// shell rather than deciding for itself.
func TestCompleteDevicePath_noDevicesMeansOrdinaryCompletion(t *testing.T) {
	where := completionPaths{workingDirectory: t.TempDir()}

	if got := completePathsIn(where, "/dev/", false); len(got) != 0 {
		t.Fatalf("with no devices reported, /dev/ offered %v", got)
	}
}

// And from the keyboard, which is where the escaping and the insertion meet the candidates.
func TestLineEditor_completesADevice(t *testing.T) {
	screen := newScreenModel(t, 70)
	editor := newLineEditor(strings.NewReader("cat /dev/zer\t\r"), screen, t.TempDir())
	editor.width = func() int { return 70 }
	editor.devices = deviceNamesFromRuntime(t)

	line, err := editor.readLine(context.Background(), "$ ")
	if err != nil {
		t.Fatalf("readLine: %v", err)
	}
	// With the trailing blank a single candidate gets, which is the existing rule -- a unique
	// match is finished, so the next word can be typed without reaching for the space bar. A
	// directory is the exception, and a device is not one.
	if line != "cat /dev/zero " {
		t.Fatalf("line = %q, want %q", line, "cat /dev/zero ")
	}
}
