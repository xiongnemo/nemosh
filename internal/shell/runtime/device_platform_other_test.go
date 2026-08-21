//go:build !windows

package runtime

import (
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// On a platform that has a real `/dev`, this shell must not invent one.
//
// The device model exists because *Windows* has no /dev. Where the platform provides one, the
// platform's is the right answer, and a synthetic eight-entry directory would shadow the genuine
// article: `ls /dev` would hide the machine's devices and `cat /dev/sda` would stop working.
//
// This test is the constraint rather than a description of it. CI made the case better than any
// argument could: with the interception still in place, the completion tests listed the real /dev on
// ubuntu and macos -- /dev/loop0, /dev/nvme0n1, three hundred ttys -- through a code path written to
// serve eight synthetic names.

func TestDevicePlatform_devIsTheSystemsOwn(t *testing.T) {
	rt := New(applets.DefaultRegistry, Streams{})
	resolved, err := rt.ResolveNemoshPath("/dev/null")
	if err != nil {
		t.Fatalf("resolving /dev/null: %v", err)
	}
	if resolved.Device {
		t.Fatal("/dev/null resolved as a shell-provided device; this platform has its own /dev")
	}
	if resolved.Native == "" {
		t.Fatal("/dev/null resolved to no native path, so nothing can open it")
	}
}

// And the directory itself is the system's, so a listing shows the machine's devices.
func TestDevicePlatform_devDirectoryIsNotSynthetic(t *testing.T) {
	rt := New(applets.DefaultRegistry, Streams{})
	resolved, err := rt.ResolveNemoshPath("/dev")
	if err != nil {
		t.Fatalf("resolving /dev: %v", err)
	}
	if resolved.Device {
		t.Fatal("/dev resolved as a synthetic directory, which would shadow the real one")
	}
	if _, ok := StatDeviceDir(string(resolved.Canonical)); ok && resolved.Native == "" {
		t.Fatal("/dev would be answered from the table rather than from the filesystem")
	}
}

// `/dev/clipboard` is a Windows facility and is not offered here. It is simply a path that does not
// exist, which is the honest answer: this shell provides nothing under a /dev the system owns.
func TestDevicePlatform_clipboardIsNotOffered(t *testing.T) {
	rt := New(applets.DefaultRegistry, Streams{})
	resolved, err := rt.ResolveNemoshPath("/dev/clipboard")
	if err != nil {
		t.Fatalf("resolving /dev/clipboard: %v", err)
	}
	if resolved.Device {
		t.Fatal("/dev/clipboard was claimed by the shell on a platform with its own /dev")
	}
	if !strings.HasSuffix(resolved.Native, "clipboard") {
		t.Fatalf("/dev/clipboard resolved to %q, want an ordinary path", resolved.Native)
	}
}

// The descriptor aliases are the exception, and the reason is that they are not devices: they name
// this shell's descriptors, which after a redirect are not the process's and which the fd table may
// hold as something that is not an operating-system file at all.
func TestDevicePlatform_descriptorAliasesAreStillTheShellsOwn(t *testing.T) {
	rt := New(applets.DefaultRegistry, Streams{})
	for _, path := range []string{"/dev/stdin", "/dev/stdout", "/dev/stderr", "/dev/fd/1"} {
		resolved, err := rt.ResolveNemoshPath(path)
		if err != nil {
			t.Fatalf("resolving %s: %v", path, err)
		}
		if !resolved.Device {
			t.Fatalf("%s resolved to the system's file; it names this shell's descriptor", path)
		}
	}
}
