package applets

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func runIpcalc(t *testing.T, lookup func(context.Context, string) ([]string, error), args ...string) (string, error) {
	t.Helper()
	saved := lookupIPName
	t.Cleanup(func() { lookupIPName = saved })
	lookupIPName = lookup
	applet, _ := DefaultRegistry.Lookup("ipcalc")
	var stdout bytes.Buffer
	ctx := WithProcessView(context.Background(), hostProcessView{cwd: t.TempDir(), environ: os.Environ()})
	err := applet.Run(ctx, args, strings.NewReader(""), &stdout, &bytes.Buffer{})
	return stdout.String(), err
}

// ipcalc's PREFIX is the bits the netmask has set, holes and all, as busybox's get_prefix counts
// them: 16 for 255.0.255.0, where it was 0. And an address with no name is busybox's `can't find
// hostname for ADDRESS`, status 1, where it was answered as its own name. Each is busybox-w32's,
// measured.
func TestIpcalc_answersAsBusyboxDoes(t *testing.T) {
	unnamed := func(context.Context, string) ([]string, error) { return nil, errors.New("No such host is known.") }
	named := func(_ context.Context, address string) ([]string, error) { return []string{"host.example."}, nil }
	if out, err := runIpcalc(t, unnamed, "-p", "1.2.3.4", "255.0.255.0"); err != nil || out != "PREFIX=16\n" {
		t.Errorf("-p with a holed netmask = %q, %v", out, err)
	}
	if out, err := runIpcalc(t, unnamed, "-p", "-m", "-n", "-b", "10.1.2.3"); err != nil || out != "NETMASK=255.0.0.0\nBROADCAST=10.255.255.255\nNETWORK=10.0.0.0\nPREFIX=8\n" {
		t.Errorf("a class A address = %q, %v", out, err)
	}
	if out, err := runIpcalc(t, named, "-h", "192.0.2.1"); err != nil || out != "HOSTNAME=host.example\n" {
		t.Errorf("-h of a named address = %q, %v", out, err)
	}
	if out, err := runIpcalc(t, unnamed, "-h", "192.0.2.1"); err == nil || out != "" || !strings.HasPrefix(err.Error(), "can't find hostname for 192.0.2.1: ") {
		t.Errorf("-h of an address with no name = %q, %v", out, err)
	}
}
