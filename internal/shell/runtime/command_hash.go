package runtime

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

// The commands the shell ran from PATH and where it found them, as busybox's ash keeps them in
// its cmdtable (shell/ash.c:9491): a command found once is not looked for again until PATH is
// assigned or `hash -r` forgets it. Every run searched, and a search tries each directory before
// the program's, six ways each on Windows: with 31 directories on PATH, 1000 runs of
// `command -v git` took 4.6 s, where busybox took 0.18 s and bash 0.26 s.
//
// The table keeps the PATH it was filled under, so whatever writes PATH without saying so -- a
// local put back, a job's state -- leaves a table that answers for another PATH, which is taken
// as empty. An entry is believed once stat says the program is still there, which busybox does
// not ask: it finds out when exec fails, and goes on along PATH. After a `cd` an entry found in
// a relative directory of PATH is looked for again, as busybox's hashcd marks them.
//
// A subshell starts with the shell's entries and changes only its own, as busybox's forked one
// does; see fork.

type statFunc func(string) (os.FileInfo, error)

type commandTable struct {
	mu sync.Mutex
	// stat is how a lookup asks about a file, os.Stat when nil. A test counts through it.
	stat statFunc
	// state is the entries, shared with the tables forked from this one until one of them is
	// changed, which copies it first; owned says this table may change it in place.
	state *commandEntries
	owned bool
}

type commandEntries struct {
	path    string
	entries map[string]hashedCommand
	added   int
}

// hashedCommand is where a command was found, which of PATH's directories that was, when it was
// added, which `hash` lists them by, and whether a `cd` since asks for it to be checked again.
type hashedCommand struct {
	native string
	index  int
	order  int
	rehash bool
}

func (t *commandTable) statter() statFunc {
	if t == nil || t.stat == nil {
		return os.Stat
	}
	return t.stat
}

// fork is the table a subshell starts with: the same entries, which neither changes in place.
func (t *commandTable) fork() *commandTable {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.owned = false
	return &commandTable{stat: t.stat, state: t.state}
}

// entry is what the table holds for name under path.
func (t *commandTable) entry(path, name string) (hashedCommand, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state == nil || t.state.path != path {
		return hashedCommand{}, false
	}
	entry, ok := t.state.entries[name]
	return entry, ok
}

// remember is name found at native, PATH's directory index, under path. A table filled under
// another PATH starts over.
func (t *commandTable) remember(path, name, native string, index int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.mutable(path)
	entry, ok := state.entries[name]
	if !ok {
		state.added++
		entry.order = state.added
	}
	entry.native, entry.index, entry.rehash = native, index, false
	state.entries[name] = entry
}

// forget is busybox's delete_cmd_entry: a name that was not found where it was.
func (t *commandTable) forget(path, name string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state == nil || t.state.path != path {
		return
	}
	if _, ok := t.state.entries[name]; ok {
		delete(t.mutable(path).entries, name)
	}
}

// reset is busybox's clearcmdentry, which `hash -r` and every assignment to PATH call.
func (t *commandTable) reset() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state, t.owned = nil, false
}

// rehash is busybox's hashcd: after a `cd` every entry is checked again before it is believed.
func (t *commandTable) rehash() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state == nil || len(t.state.entries) == 0 {
		return
	}
	state := t.mutable(t.state.path)
	for name, entry := range state.entries {
		entry.rehash = true
		state.entries[name] = entry
	}
}

// mutable is the entries for path that this table may change, copied if they are shared.
func (t *commandTable) mutable(path string) *commandEntries {
	switch {
	case t.state == nil || t.state.path != path:
		t.state = &commandEntries{path: path, entries: map[string]hashedCommand{}}
	case !t.owned:
		t.state = &commandEntries{path: path, entries: maps.Clone(t.state.entries), added: t.state.added}
	}
	t.owned = true
	return t.state
}

// listing is busybox's `hash` with no operand: a line for each entry, the path as `command -v`
// prints it, starred when a `cd` has asked for it to be checked again, in the order busybox's
// table holds them -- by the bucket its hash puts a name in (cmdlookup_pp), and within one in
// the order they were added.
func (t *commandTable) listing(path string) []string {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state == nil || t.state.path != path {
		return nil
	}
	names := slices.Collect(maps.Keys(t.state.entries))
	slices.SortFunc(names, func(a, b string) int {
		if bucket, other := ashCommandBucket(a), ashCommandBucket(b); bucket != other {
			return bucket - other
		}
		return t.state.entries[a].order - t.state.entries[b].order
	})
	lines := make([]string, 0, len(names))
	for _, name := range names {
		entry := t.state.entries[name]
		line := filepath.ToSlash(entry.native)
		if entry.rehash {
			line += "*"
		}
		lines = append(lines, line)
	}
	return lines
}

// withProgramPATH is the shell a command with assignments of its own runs in: one whose table
// is its own when it is given a PATH and is a program. busybox puts a program's assignments in
// its environment and looks it up without the table, so `PATH=/x git` leaves the shell's alone;
// a builtin's or a function's it assigns for their while, which forgets the table as any
// assignment to PATH does.
func (r Runtime) withProgramPATH(name string, assignments []assignment) Runtime {
	if r.commands == nil || isRuntimeBuiltin(name) || !slices.ContainsFunc(assignments, assignsPATH) {
		return r
	}
	if _, function := r.calledFunction(name); !function {
		r.detach()
		r.commands = r.commands.fork()
	}
	return r
}

func assignsPATH(assignment assignment) bool { return assignment.name == "PATH" }

// ashCommandBucket is the bucket of busybox's cmdtable a name falls in (shell/ash.c:9561-9566).
func ashCommandBucket(name string) int {
	if name == "" {
		return 0
	}
	value := uint(name[0]) << 4
	for index := range len(name) {
		value += uint(name[index])
	}
	return int(value&0x7fff) % 31
}
