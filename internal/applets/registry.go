package applets

import (
	"context"
	"io"
	"path/filepath"
	"strings"
)

type Applet interface {
	Name() string
	Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error
}

type Registry struct {
	applets map[string]Applet
}

func NewRegistry(items ...Applet) Registry {
	r := Registry{applets: make(map[string]Applet, len(items))}
	for _, item := range items {
		r.applets[item.Name()] = item
	}
	return r
}

func (r Registry) Lookup(name string) (Applet, bool) {
	applet, ok := r.applets[name]
	return applet, ok
}

func InvocationName(args []string) string {
	if len(args) == 0 || args[0] == "" {
		return ""
	}
	base := filepath.Base(args[0])
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return strings.ToLower(base)
}

var DefaultRegistry = NewRegistry(
	newTrueApplet(),
	newFalseApplet(),
	newEchoApplet(),
	newPrintfApplet(),
	newCatApplet(),
	newPwdApplet(),
	newHeadApplet(),
	newTailApplet(),
	newWcApplet(),
	newEnvApplet(),
	newPrintenvApplet(),
	newTestApplet("test"),
	newTestApplet("["),
	newBasenameApplet(),
	newDirnameApplet(),
	newTouchApplet(),
	newRmApplet(),
	newMkdirApplet(),
	newRmdirApplet(),
	newLsApplet(),
	newCpApplet(),
	newMvApplet(),
	newGrepApplet(),
)
