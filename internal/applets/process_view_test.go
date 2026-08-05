package applets

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

func TestP05WaveA_ResolveProcessPath_propagatesTypedResolutionError(t *testing.T) {
	// Given
	wantErr := errors.New("path resolution failed")
	view := typedPathTestView{legacyPathTestView: legacyPathTestView{}, err: wantErr}

	// When
	_, err := ResolveProcessPath(view, "/cygdrive/c/file")

	// Then
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected typed resolver error %v, got %v", wantErr, err)
	}
}

func TestP05WaveA_ResolveProcessPath_wrapsLegacyViewAsHostTarget(t *testing.T) {
	// Given
	native := filepath.Join("host", "child.txt")
	view := legacyPathTestView{resolved: native}

	// When
	resolved, err := ResolveProcessPath(view, "child.txt")

	// Then
	if err != nil {
		t.Fatalf("expected legacy resolution to succeed, got %v", err)
	}
	if resolved.Native != native || resolved.Device {
		t.Fatalf("unexpected legacy resolution: %+v", resolved)
	}
	if resolved.Canonical != pathmodel.Path(filepath.ToSlash(native)) {
		t.Fatalf("expected canonical path %q, got %q", filepath.ToSlash(native), resolved.Canonical)
	}
}

func TestStaticProcessView_overlaysAssignmentsWithoutMutatingParent(t *testing.T) {
	// Given
	parent := environmentTestView{items: []string{"KEEP=parent", "CHANGE=parent"}}
	view := newStaticProcessView(parent, parent.Environ())

	// When
	view.set("CHANGE", "child")
	view.set("CHILD", "value")

	// Then
	if got, ok := view.LookupEnv("CHANGE"); !ok || got != "child" {
		t.Fatalf("child CHANGE: value=%q present=%t", got, ok)
	}
	if got, ok := view.LookupEnv("CHILD"); !ok || got != "value" {
		t.Fatalf("child CHILD: value=%q present=%t", got, ok)
	}
	if got, ok := parent.LookupEnv("CHANGE"); !ok || got != "parent" {
		t.Fatalf("parent CHANGE: value=%q present=%t", got, ok)
	}
	if _, ok := parent.LookupEnv("CHILD"); ok {
		t.Fatal("child assignment leaked into parent")
	}
}

func TestStaticProcessView_preservesDistinctExactCaseNames(t *testing.T) {
	// Given
	view := newStaticProcessView(environmentTestView{}, []string{"Path=title", "PATH=upper"})
	view.set("Path", "latest")
	view.set("EMPTY", "")

	// When
	pathTitle, titleOK := view.LookupEnv("Path")
	pathUpper, upperOK := view.LookupEnv("PATH")

	// Then
	if !titleOK || pathTitle != "latest" {
		t.Fatalf("Path: value=%q present=%t", pathTitle, titleOK)
	}
	if !upperOK || pathUpper != "upper" {
		t.Fatalf("PATH: value=%q present=%t", pathUpper, upperOK)
	}
	if empty, ok := view.LookupEnv("EMPTY"); !ok || empty != "" {
		t.Fatalf("EMPTY: value=%q present=%t", empty, ok)
	}
	want := []string{"EMPTY=", "PATH=upper", "Path=latest"}
	if got := view.Environ(); !reflect.DeepEqual(got, want) {
		t.Fatalf("environment: got %v want %v", got, want)
	}
}

type legacyPathTestView struct {
	resolved string
}

func (legacyPathTestView) WorkingDirectory() string        { return "." }
func (legacyPathTestView) Environ() []string               { return nil }
func (legacyPathTestView) LookupEnv(string) (string, bool) { return "", false }
func (v legacyPathTestView) ResolvePath(string) string     { return v.resolved }

type typedPathTestView struct {
	legacyPathTestView
	err error
}

func (v typedPathTestView) ResolveNemoshPath(string) (pathmodel.ResolvedPath, error) {
	return pathmodel.ResolvedPath{}, v.err
}

type environmentTestView struct {
	items []string
}

func (environmentTestView) WorkingDirectory() string       { return "." }
func (v environmentTestView) Environ() []string            { return append([]string(nil), v.items...) }
func (environmentTestView) ResolvePath(path string) string { return path }
func (v environmentTestView) LookupEnv(name string) (string, bool) {
	for _, item := range v.items {
		itemName, value, found := strings.Cut(item, "=")
		if found && itemName == name {
			return value, true
		}
	}
	return "", false
}
