package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// pathIndex is every program on PATH, by name.
//
// It exists because a command that runs must not be drawn as a command that
// does not. `wsl` is on PATH and was drawn red, which is the shell contradicting
// itself: press Enter and it works.
//
// Reading PATH cannot go on the keystroke path. Measured on the machine this was
// written on, 78 directories holding 9,917 files take 16ms to walk -- nothing
// once, and far too much per character. So it is walked once, in the background,
// and until it answers nothing is claimed either way. A name is drawn plainly
// while the index is still building rather than being called an error and then
// turning green a moment later.
type pathIndex struct {
	mu      sync.RWMutex
	ready   bool
	builtOn string
	// inFlight is a flag rather than "building is non-empty", because an empty
	// PATH is a real value: comparing the strings alone made the zero value look
	// like a build already under way, and the index then never built at all.
	inFlight bool
	building string
	names    map[string]bool
	sorted   []string
}

func newPathIndex() *pathIndex {
	return &pathIndex{names: map[string]bool{}}
}

// refresh rebuilds in the background if PATH has changed since the last build.
//
// Comparing the PATH string is the whole invalidation story, and it is the right
// one for a shell: a session that installs a program and expects the prompt to
// notice is rare, while a session that changes PATH and expects that to take
// effect is ordinary. Rebuilding costs 16ms and only happens when the variable
// actually moves.
func (p *pathIndex) refresh(pathValue string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready && p.builtOn == pathValue {
		return
	}
	if p.inFlight && p.building == pathValue {
		return
	}
	p.inFlight, p.building = true, pathValue

	go func() {
		names, sorted := scanPath(pathValue)
		p.mu.Lock()
		defer p.mu.Unlock()
		p.names, p.sorted = names, sorted
		p.builtOn, p.ready, p.inFlight = pathValue, true, false
	}()
}

// has reports whether PATH holds the name, and whether the index can answer at
// all yet. Two booleans rather than one, because "not found" and "not looked
// yet" must not be drawn the same way.
func (p *pathIndex) has(name string) (found, ready bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.ready {
		return false, false
	}
	return p.names[indexKey(name)], true
}

// builtFrom is the PATH the current index was read from, which is how a caller
// tells one build from the next. Behind the lock, like every other field: the
// build runs on its own goroutine and nothing here may be read bare.
func (p *pathIndex) builtFrom() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.builtOn
}

// candidates is the sorted names, for the suggestion engine. Empty until the
// index is ready, which simply means no suggestion comes from PATH yet.
func (p *pathIndex) candidates() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sorted
}

// scanPath reads every directory on PATH and returns the names that can be run,
// and separately the names worth offering.
//
// The two are not the same list, which is the point. `wsl.exe` must be
// *recognised* under both spellings, because either can be typed -- but only
// `wsl` should be *offered*, because completing `w` into a column containing
// both `wsl` and `wsl.exe` reads as though there were two programs.
func scanPath(pathValue string) (map[string]bool, []string) {
	names := map[string]bool{}
	var sorted []string
	seen := map[string]bool{}
	// recognise makes a spelling answerable; offer also proposes it.
	recognise := func(name string) {
		names[indexKey(name)] = true
	}
	offer := func(name string) {
		recognise(name)
		if key := indexKey(name); !seen[key] {
			seen[key] = true
			sorted = append(sorted, name)
		}
	}
	for _, directory := range filepath.SplitList(pathValue) {
		if directory == "" {
			continue
		}
		entries, err := os.ReadDir(directory)
		if err != nil {
			// A directory on PATH that cannot be read is ordinary -- a removable
			// drive, a stale entry -- and is not worth a diagnostic on a path
			// nobody asked about.
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if stem, ok := executableStem(name); ok {
				offer(stem)
				recognise(name)
				continue
			}
			if !runsWithoutASuffix {
				continue
			}
			offer(name)
		}
	}
	sort.Strings(sorted)
	return names, sorted
}

// executableStem strips a Windows executable suffix, reporting whether the name
// had one.
func executableStem(name string) (string, bool) {
	for _, suffix := range executableSuffixes {
		if len(name) > len(suffix) && strings.EqualFold(name[len(name)-len(suffix):], suffix) {
			return name[:len(name)-len(suffix)], true
		}
	}
	return name, false
}

// The suffixes this shell will launch, matching the lookup order in
// internal/shell/runtime/external.go.
var executableSuffixes = []string{".com", ".exe", ".bat", ".cmd"}
