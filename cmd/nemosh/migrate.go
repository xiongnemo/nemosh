package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// `nemosh --migrate [--dry-run]` moves what a release up to 1.5 kept to where later ones keep
// it, in the XDG layout under the home directory (xdg_dirs.go), as every tool of the author's has
// a migrate of its own. --dry-run prints the plan and touches nothing. Nothing is overwritten:
// the same file on both sides leaves the new one and removes the old, a different one keeps
// both, says so, and ends with status 1. A second run finds nothing to do. ~/.nemoshrc is the rc
// file, which stays where it is.

// migrateItem is one thing that moves, from where an older release kept it to where it goes.
type migrateItem struct {
	what, from, to string
}

// migrateRun is one migration: where it looks, what it says, and how it asks for the rest.
type migrateRun struct {
	lookup         func(string) (string, bool)
	stdout, stderr io.Writer
	// others is the other nemosh processes running, which are refused; see otherShells.
	others func() ([]int, error)
	// rename is os.Rename, which a test makes fail across volumes.
	rename    func(string, string) error
	dryRun    bool
	conflicts int
}

func (c command) migrate(args []string) error {
	run := migrateRun{lookup: os.LookupEnv, stdout: c.stdout, stderr: c.stderr, others: otherShells, rename: os.Rename}
	return run.run(args)
}

func (m *migrateRun) run(args []string) error {
	for _, arg := range args {
		if arg != "--dry-run" {
			fmt.Fprintf(m.stderr, "nemosh: --migrate takes only --dry-run, not %s\n", arg)
			return exitStatus(2)
		}
		m.dryRun = true
	}
	var found []migrateItem
	for _, item := range m.items() {
		if _, err := os.Stat(item.from); err == nil {
			found = append(found, item)
		}
	}
	if len(found) == 0 {
		fmt.Fprintln(m.stdout, "nemosh: nothing to migrate")
		return nil
	}
	if others, err := m.others(); err == nil && len(others) > 0 {
		text := make([]string, len(others))
		for index, pid := range others {
			text[index] = strconv.Itoa(pid)
		}
		if !m.dryRun {
			fmt.Fprintf(m.stderr, "nemosh: --migrate: other nemosh processes are running (%s), and would go on writing where the files were; close them and run it again\n", strings.Join(text, ", "))
			return exitStatus(1)
		}
		fmt.Fprintf(m.stdout, "nemosh: other nemosh processes are running (%s), so the migration itself would be refused until they end\n", strings.Join(text, ", "))
	}
	for _, item := range found {
		m.moveFile(item.what, item.from, item.to)
	}
	if !m.dryRun {
		fmt.Fprintln(m.stdout, "nemosh: the shell this was run from goes on using the old places until it exits; a new one uses the new")
	}
	if m.conflicts > 0 {
		fmt.Fprintf(m.stderr, "nemosh: --migrate: %d conflict(s), each kept on both sides\n", m.conflicts)
		return exitStatus(1)
	}
	return nil
}

// items is everything that may need moving, in native paths.
func (m *migrateRun) items() []migrateItem {
	home := migrateHome(m.lookup)
	if home == "" {
		return nil
	}
	homed := func(name string) (string, bool) {
		if name == "HOME" {
			return home, true
		}
		return m.lookup(name)
	}
	current, legacy := historyPaths(homed)
	return []migrateItem{{what: "history", from: filepath.FromSlash(legacy), to: filepath.FromSlash(current)}}
}

// migrateHome is HOME when it is a native absolute path, and USERPROFILE otherwise, which is
// where a session's HOME comes from when it is not set (login_defaults_windows.go).
func migrateHome(lookup func(string) (string, bool)) string {
	if home, ok := lookup("HOME"); ok && filepath.IsAbs(home) {
		return home
	}
	if profile, ok := lookup("USERPROFILE"); ok && filepath.IsAbs(profile) {
		return profile
	}
	return ""
}

// say prints one action, or with --dry-run the action it would take.
func (m *migrateRun) say(done, planned, from, to string) {
	verb := done
	if m.dryRun {
		verb = planned
	}
	fmt.Fprintf(m.stdout, "%s %s -> %s\n", verb, filepath.ToSlash(from), filepath.ToSlash(to))
}

// otherShells is the nemosh processes running besides this one and the ones that started it --
// the shell it was typed in, and a Scoop shim between them.
func otherShells() ([]int, error) {
	processes, err := proc.List()
	if err != nil {
		return nil, err
	}
	byPID := make(map[int]proc.Process, len(processes))
	for _, process := range processes {
		byPID[process.PID] = process
	}
	ancestors := map[int]bool{}
	pid, born := os.Getppid(), time.Now()
	for range 32 {
		// A parent started after its child is a pid used again, which Windows does.
		parent, ok := byPID[pid]
		if !ok || parent.Created.After(born) {
			break
		}
		ancestors[pid] = true
		pid, born = parent.PPID, parent.Created
	}
	var others []int
	for _, process := range processes {
		if strings.EqualFold(process.Name, "nemosh.exe") && !ancestors[process.PID] {
			others = append(others, process.PID)
		}
	}
	sort.Ints(others)
	return others, nil
}
