package runtime

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
)

type WorkingDirectory string

type Environment struct {
	values map[string]environmentValue
	next   *uint64
}

type environmentValue struct {
	name  string
	value string
	order uint64
}

type State struct {
	// Cwd is an absolute native directory. Windows also accepts explicit drive,
	// UNC, and configured drive-alias spellings; virtual roots need root provenance.
	Cwd   WorkingDirectory
	Env   Environment
	Paths *PathSettings
}

var (
	ErrStateCwdRequired             = errors.New("runtime state cwd is required")
	ErrStateCwdNotAbsolute          = errors.New("runtime state cwd is not absolute")
	ErrStateCwdNeedsRoot            = errors.New("runtime state virtual cwd requires an explicit current root")
	ErrStateTmpRootRequired         = errors.New("runtime state tmp root is required")
	ErrStateTmpRootNotAbsolute      = errors.New("runtime state tmp root is not absolute")
	ErrStateMountPrefixNotAbsolute  = errors.New("runtime state mount prefix is not absolute")
	ErrStateMountPrefixNotCanonical = errors.New("runtime state mount prefix is not canonical")
	ErrStateMountPrefixNeedsSegment = errors.New("runtime state mount prefix must name a non-root segment")
)

func NewEnvironment(items []string) Environment {
	next := uint64(0)
	env := Environment{values: make(map[string]environmentValue, len(items)), next: &next}
	for _, item := range items {
		name, value, found := strings.Cut(item, "=")
		if found {
			env.Set(name, value)
		}
	}
	return env
}

func (e *Environment) initialize() {
	if e.values == nil {
		e.values = make(map[string]environmentValue)
	}
	if e.next == nil {
		next := uint64(0)
		e.next = &next
	}
}

func (e *Environment) Set(name, value string) {
	e.initialize()
	(*e.next)++
	e.values[name] = environmentValue{name: name, value: value, order: *e.next}
}
func (e *Environment) Unset(name string) {
	e.initialize()
	(*e.next)++
	delete(e.values, name)
}
func (e Environment) LookupEnv(name string) (string, bool) {
	value, ok := e.values[name]
	return value.value, ok
}
func (e Environment) Environ() []string {
	items := make([]string, 0, len(e.values))
	for _, value := range e.values {
		items = append(items, value.name+"="+value.value)
	}
	sort.Strings(items)
	return items
}
func (e Environment) clone() Environment {
	values := make(map[string]environmentValue, len(e.values))
	maps.Copy(values, e.values)
	next := uint64(0)
	if e.next != nil {
		next = *e.next
	}
	return Environment{values: values, next: &next}
}

func hostState() State {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return State{Cwd: WorkingDirectory(cwd), Env: newHostEnvironment(os.Environ(), hostEnvironmentPlatform())}
}

func NewWithState(registry applets.Registry, streams Streams, state State) Runtime {
	runtime, err := NewRuntimeWithState(registry, streams, state)
	if err == nil {
		return runtime
	}
	return newRuntimeWithState(registry, streams, state, pathState{}, err)
}

func NewRuntimeWithState(registry applets.Registry, streams Streams, state State) (Runtime, error) {
	settings := pathSettings(state)
	if err := validateInitialCwd(state.Cwd, settings); err != nil {
		return Runtime{}, fmt.Errorf("initialize runtime state: %w", err)
	}
	if err := validateTmpRoot(settings); err != nil {
		return Runtime{}, fmt.Errorf("initialize runtime state: %w", err)
	}
	if err := validateMountPrefix(settings); err != nil {
		return Runtime{}, fmt.Errorf("initialize runtime state: %w", err)
	}
	paths := newPathState(state.Cwd, settings)
	nativeCwd, err := paths.nativeWorkingDirectory()
	if err != nil {
		return Runtime{}, fmt.Errorf("initialize runtime cwd %q: %w", state.Cwd, err)
	}
	if !filepath.IsAbs(nativeCwd) {
		return Runtime{}, fmt.Errorf("initialize runtime cwd %q: %w", state.Cwd, ErrStateCwdNotAbsolute)
	}
	return newRuntimeWithState(registry, streams, state, paths, nil), nil
}

func pathSettings(state State) PathSettings {
	if state.Paths != nil {
		return *state.Paths
	}
	return DefaultPathSettings()
}

func validateTmpRoot(settings PathSettings) error {
	if !settings.Config.EnableTmp {
		return nil
	}
	if settings.TmpRoot == "" {
		return ErrStateTmpRootRequired
	}
	if !filepath.IsAbs(string(settings.TmpRoot)) {
		return fmt.Errorf("tmp root %q: %w", settings.TmpRoot, ErrStateTmpRootNotAbsolute)
	}
	return nil
}

func validateMountPrefix(settings PathSettings) error {
	prefix := settings.Config.MountPrefix
	if !settings.Config.EnableMountPath || prefix == "" {
		return nil
	}
	if !path.IsAbs(prefix) {
		return fmt.Errorf("mount prefix %q: %w", prefix, ErrStateMountPrefixNotAbsolute)
	}
	if prefix == "/" {
		return fmt.Errorf("mount prefix %q: %w", prefix, ErrStateMountPrefixNeedsSegment)
	}
	if path.Clean(prefix) != prefix {
		return fmt.Errorf("mount prefix %q: %w", prefix, ErrStateMountPrefixNotCanonical)
	}
	return nil
}

func newRuntimeWithState(registry applets.Registry, streams Streams, state State, paths pathState, initErr error) Runtime {
	fds := newFDTable(streams)
	variables := make(map[string]string, len(state.Env.values))
	for _, value := range state.Env.values {
		variables[value.name] = value.value
	}
	created := Runtime{initErr: initErr, registry: registry, functions: map[functionName]functionDefinition{}, streams: fds.streams(), fds: fds, vars: variables, traps: map[trapName]string{}, trapRunning: map[trapName]bool{}, params: &parameters{}, options: &shellOptions{extGlob: true}, expansion: newExpansionState(), aliases: map[string]string{}, childCPU: &childCPUTime{}, history: newShellHistory(), arrays: newShellArrays(), loops: newLoopLevels(), special: newSpecialState(), readonly: map[string]struct{}{}, mask: newFileModeMask(), paths: &paths, env: state.Env.clone(), jobScope: newRootJobScope(), lifecycle: &shellLifecycle{}}
	// $PWD has to answer for this shell's working directory rather than for
	// whatever launched it. Nemosh's cwd is a value in pathState, not the
	// process's, so an inherited PWD can be wrong from the very first line.
	if initErr == nil {
		created.setDirectoryVariable("PWD", created.WorkingDirectory())
		// Windows sets none of the variables a shell assumes it was logged in
		// with, and the symptoms of that do not look like one cause. See
		// login_defaults.go.
		created.applyLoginDefaults()
	}
	return created
}

func (r Runtime) WorkingDirectory() string {
	if r.initErr != nil {
		return ""
	}
	return r.paths.workingDirectory()
}
func (r Runtime) Environ() []string                    { return r.env.Environ() }
func (r Runtime) LookupEnv(name string) (string, bool) { return r.env.LookupEnv(name) }
func (r Runtime) LookupVariable(name string) (string, bool) {
	value, present := r.vars[name]
	return value, present
}
func (r Runtime) ResolvePath(path string) string { return r.resolvePath(path) }

// RunnableNames is every name this session can run that is not a builtin or an
// applet: the aliases and functions it has defined.
//
// The line editor asks so that it can draw a name it can run in the colour of a
// name it can run. Without it, `ll` from an rc file was drawn as an error while
// working perfectly, which is the shell contradicting itself on screen.
func (r Runtime) RunnableNames() []string {
	names := make([]string, 0, len(r.aliases)+len(r.functions))
	for name := range r.aliases {
		names = append(names, name)
	}
	for name := range r.functions {
		names = append(names, name.String())
	}
	sort.Strings(names)
	return names
}
