package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

type ProcessView interface {
	WorkingDirectory() string
	Environ() []string
	LookupEnv(string) (string, bool)
	ResolvePath(string) string
}

type pathProcessView interface {
	ResolveNemoshPath(string) (pathmodel.ResolvedPath, error)
}

type processInputView interface {
	OpenProcessInput(string) (io.ReadCloser, error)
}

func ResolveProcessPath(view ProcessView, path string) (pathmodel.ResolvedPath, error) {
	if resolver, ok := view.(pathProcessView); ok {
		return resolver.ResolveNemoshPath(path)
	}
	native := view.ResolvePath(path)
	return pathmodel.ResolvedPath{Canonical: pathmodel.Path(filepath.ToSlash(native)), Native: native}, nil
}

func resolveHostPath(view ProcessView, path string) (string, error) {
	resolved, err := ResolveProcessPath(view, path)
	if err != nil {
		return "", err
	}
	if resolved.Device {
		return "", fmt.Errorf("%s is not a host path", resolved.Canonical)
	}
	return resolved.Native, nil
}

func OpenProcessInput(ctx context.Context, view ProcessView, path string) (io.ReadCloser, error) {
	input, err := openProcessInput(view, path)
	if err != nil {
		return nil, err
	}
	return &contextReadCloser{ctx: ctx, input: input}, nil
}

func openProcessInput(view ProcessView, path string) (io.ReadCloser, error) {
	if opener, ok := view.(processInputView); ok {
		return opener.OpenProcessInput(path)
	}
	native, err := resolveHostPath(view, path)
	if err != nil {
		return nil, err
	}
	return OpenHostInput(native)
}

// OpenHostInput opens a resolved host path for reading. It is exported because
// the runtime resolves device paths itself and would otherwise reach os.Open
// directly, skipping the check below.
//
// Unlike POSIX open(2), Windows lets Go open a directory; the failure only
// surfaces at the first Read, as ERROR_INVALID_FUNCTION ("Incorrect function"),
// naming the host path. busybox-w32 reports EISDIR wherever it can tell a
// directory apart (win32/mingw.c:316), and having opened the handle we always
// can, so every reader refuses a directory up front and with one wording.
func OpenHostInput(native string) (io.ReadCloser, error) {
	file, err := os.Open(native)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if info.IsDir() {
		isDir := &fs.PathError{Op: "open", Path: native, Err: syscall.EISDIR}
		return nil, errors.Join(isDir, file.Close())
	}
	return file, nil
}

type contextReadCloser struct {
	ctx      context.Context
	input    io.ReadCloser
	close    sync.Once
	closeErr error
}

func (r *contextReadCloser) Read(buffer []byte) (int, error) {
	return readWithContext(r.ctx, r.input, buffer)
}

func (r *contextReadCloser) ReadContext(ctx context.Context, buffer []byte) (int, error) {
	return readWithContext(ctx, r.input, buffer)
}

func (r *contextReadCloser) Close() error {
	r.close.Do(func() { r.closeErr = r.input.Close() })
	return r.closeErr
}

func canonicalizeGeneratedPath(view ProcessView, origin pathmodel.Path, native string) string {
	if resolver, ok := view.(interface {
		CanonicalizeNativePath(pathmodel.Path, string) (pathmodel.Path, error)
	}); ok {
		if canonical, err := resolver.CanonicalizeNativePath(origin, native); err == nil {
			return string(canonical)
		}
	}
	return filepath.ToSlash(native)
}

type processViewKey struct{}

func WithProcessView(ctx context.Context, view ProcessView) context.Context {
	return context.WithValue(ctx, processViewKey{}, view)
}

func ProcessViewFromContext(ctx context.Context) ProcessView {
	if view, ok := ctx.Value(processViewKey{}).(ProcessView); ok {
		return view
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return hostProcessView{cwd: cwd, environ: os.Environ()}
}

type hostProcessView struct {
	cwd     string
	environ []string
}

type staticProcessView struct {
	parent ProcessView
	values map[string]environmentEntry
}

type environmentEntry struct{ name, value string }

func newStaticProcessView(parent ProcessView, items []string) staticProcessView {
	v := staticProcessView{parent: parent, values: make(map[string]environmentEntry, len(items))}
	for _, item := range items {
		name, value, found := strings.Cut(item, "=")
		if found {
			v.set(name, value)
		}
	}
	return v
}

func (v staticProcessView) set(name, value string) {
	v.values[name] = environmentEntry{name: name, value: value}
}

func (v staticProcessView) WorkingDirectory() string { return v.parent.WorkingDirectory() }
func (v staticProcessView) ResolvePath(path string) string {
	return v.parent.ResolvePath(path)
}
func (v staticProcessView) ResolveNemoshPath(path string) (pathmodel.ResolvedPath, error) {
	return ResolveProcessPath(v.parent, path)
}
func (v staticProcessView) OpenProcessInput(path string) (io.ReadCloser, error) {
	return openProcessInput(v.parent, path)
}
func (v staticProcessView) CanonicalizeNativePath(origin pathmodel.Path, path string) (pathmodel.Path, error) {
	if resolver, ok := v.parent.(interface {
		CanonicalizeNativePath(pathmodel.Path, string) (pathmodel.Path, error)
	}); ok {
		return resolver.CanonicalizeNativePath(origin, path)
	}
	return pathmodel.Path(filepath.ToSlash(path)), nil
}
func (v staticProcessView) Environ() []string {
	items := make([]string, 0, len(v.values))
	for _, entry := range v.values {
		items = append(items, entry.name+"="+entry.value)
	}
	sort.Strings(items)
	return items
}
func (v staticProcessView) LookupEnv(name string) (string, bool) {
	entry, ok := v.values[name]
	return entry.value, ok
}

func (v hostProcessView) WorkingDirectory() string { return v.cwd }
func (v hostProcessView) Environ() []string        { return append([]string(nil), v.environ...) }
func (v hostProcessView) ResolvePath(path string) string {
	return resolveViewPath(v.cwd, path)
}
func (v hostProcessView) LookupEnv(name string) (string, bool) {
	for _, item := range v.environ {
		key, value, _ := strings.Cut(item, "=")
		if environmentNamesEqual(key, name) {
			return value, true
		}
	}
	return "", false
}

func environmentNamesEqual(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func resolveViewPath(cwd, path string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return path
	}
	if strings.HasSuffix(cwd, "/") || strings.HasSuffix(cwd, `\`) {
		return cwd + path
	}
	return cwd + string(os.PathSeparator) + path
}

// processStatView is what a view implements when it can describe a device.
//
// The mirror of processInputView above, and for the same reason: an applet asks the view rather than
// carrying its own idea of which names are devices. The runtime's implementation is
// runtime.Runtime.StatProcessPath.
type processStatView interface {
	StatProcessPath(string) (fs.FileInfo, bool, error)
}

// statProcessPath stats a path, answering from the device table where the path names a device.
//
// keepLink is Lstat rather than Stat, which is what `test -h` and `test -L` want: a symlink reported
// as a symlink rather than as whatever it points at. It was called followLink and meant the
// opposite, which is the sort of name that survives until somebody adds a caller.
//
// Devices have no links, so the flag only reaches the filesystem branch.
func statProcessPath(view ProcessView, path string, keepLink bool) (fs.FileInfo, error) {
	if stat, ok := view.(processStatView); ok {
		info, isDevice, err := stat.StatProcessPath(path)
		if err != nil {
			return nil, err
		}
		if isDevice {
			return info, nil
		}
	}
	native, err := resolveHostPath(view, path)
	if err != nil {
		return nil, err
	}
	if keepLink {
		return os.Lstat(native)
	}
	return os.Stat(native)
}

// statDeviceOperand describes an operand that names a device, and answers nil for anything else.
//
// Separate from statProcessPath because the callers differ in what they want from a non-device: this
// one hands the question back so the caller can go on to resolve a host path its own way, which is
// what `ls` needs -- it lists a directory rather than stat-ing a file.
func statDeviceOperand(view ProcessView, path string) (fs.FileInfo, error) {
	stat, ok := view.(processStatView)
	if !ok {
		return nil, nil
	}
	info, isDevice, err := stat.StatProcessPath(path)
	if err != nil || !isDevice {
		return nil, err
	}
	return info, nil
}
