//go:build windows

package runtime

import (
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Windows has no /dev, so the shell provides one.

func TestDevicePlatform_devIsProvidedByTheShell(t *testing.T) {
	rt := New(applets.DefaultRegistry, Streams{})
	for _, path := range []string{"/dev", "/dev/null", "/dev/clipboard"} {
		resolved, err := rt.ResolveNemoshPath(path)
		if err != nil {
			t.Fatalf("resolving %s: %v", path, err)
		}
		if !resolved.Device {
			t.Fatalf("%s did not resolve as a shell-provided device; Windows has no /dev of its own", path)
		}
		if resolved.Native != "" {
			t.Fatalf("%s resolved to native %q; a device has no host path", path, resolved.Native)
		}
	}
}
