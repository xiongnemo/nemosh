package applets

import (
	"context"
	"io"
	"os"
	"reflect"
)

// ContextReader is an applet's stdin as the registry hands it over: reads that end when
// Ctx is cancelled. The registry wraps every applet's stdin in one (contextApplet), which
// is an allocation per call; a caller that keeps its own for the call -- the shell does,
// in a frame a repeated command is handed again, so running one allocates nothing --
// passes that instead, and the registry does not wrap it a second time.
type ContextReader struct {
	Ctx    context.Context
	Reader io.Reader
}

func (r *ContextReader) Read(buffer []byte) (int, error) {
	return readWithContext(r.Ctx, r.Reader, buffer)
}

func (r *ContextReader) ReadContext(ctx context.Context, buffer []byte) (int, error) {
	return readWithContext(ctx, r.Reader, buffer)
}

// LeaseStdinFile passes the request through, as contextReader's does; see there.
func (r *ContextReader) LeaseStdinFile(ctx context.Context) (*os.File, func(), bool) {
	return contextReader{ctx: r.Ctx, reader: r.Reader}.LeaseStdinFile(ctx)
}

// readerFor is stdin as an applet called in ctx should get it: as it is, when it is
// already a ContextReader for that same context, and wrapped otherwise.
func readerFor(ctx context.Context, stdin io.Reader) io.Reader {
	if wrapped, ok := stdin.(*ContextReader); ok && SameContext(wrapped.Ctx, ctx) {
		return stdin
	}
	return contextReader{ctx: ctx, reader: stdin}
}

// SameContext reports whether a and b are one context. Not a bare ==, which panics when
// both hold one uncomparable type; reflect.TypeOf allocates nothing.
func SameContext(a, b context.Context) bool {
	kind := reflect.TypeOf(a)
	return kind != nil && kind == reflect.TypeOf(b) && kind.Comparable() && a == b
}

// IsShellKey reports whether key is one of the two the shell answers in an applet's
// context: its ProcessView and its ProgramRunner. A context that stands for the shell
// without being made by WithProcessView and WithProgramRunner -- the shell's call frame,
// which a repeated command is handed again rather than allocating -- answers these keys
// from its Value.
func IsShellKey(key any) bool {
	switch key.(type) {
	case processViewKey, programRunnerKey:
		return true
	}
	return false
}
