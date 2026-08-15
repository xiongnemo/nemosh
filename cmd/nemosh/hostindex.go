package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// hostIndex is the machines this session could reasonably be asked to connect
// to, by name.
//
// The same shape as pathIndex, and for the same reason: a suggestion is computed
// after every keystroke, so every source it consults has to already be in
// memory. Reading a config file per character is felt, and a shell that stutters
// as you type is worse than one that suggests nothing.
//
// The sources are deliberately few. `~/.ssh/config` is the file a person curates
// on purpose, so its names are the ones they meant. `known_hosts` is *not* read:
// it is a machine-written cache, it is the file most likely to be enormous, and
// under `HashKnownHosts yes` -- OpenSSH's own default on many distributions --
// its entries are hashed and unreadable by design, so it would answer richly on
// one machine and not at all on the next.
type hostIndex struct {
	mu       sync.RWMutex
	ready    bool
	inFlight bool
	builtOn  string
	building string
	sorted   []string
}

func newHostIndex() *hostIndex {
	return &hostIndex{}
}

// refresh rebuilds in the background when a source file has changed.
//
// PATH could be compared as a string because it *is* the input. A file has no
// such handle, so the fingerprint is its size and modification time -- which is
// what stat already knows, so noticing costs one syscall per source per prompt
// and nothing at all per keystroke.
//
// Rebuilding on change rather than once per session is the difference that
// matters in use: editing `~/.ssh/config` and immediately using the new name is
// the ordinary loop, and a shell that needed restarting for it would be
// answering about a file that no longer exists.
func (h *hostIndex) refresh(sources []string) {
	fingerprint := fingerprintFiles(sources)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.ready && h.builtOn == fingerprint {
		return
	}
	if h.inFlight && h.building == fingerprint {
		return
	}
	h.inFlight, h.building = true, fingerprint

	go func() {
		names := scanHostSources(sources)
		h.mu.Lock()
		defer h.mu.Unlock()
		h.sorted = names
		h.builtOn, h.ready, h.inFlight = fingerprint, true, false
	}()
}

// candidates is the sorted names. Empty until the first build finishes, which
// simply means no host is offered yet -- the same answer as having none.
func (h *hostIndex) candidates() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sorted
}

func (h *hostIndex) builtFrom() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.builtOn
}

// hostSources is where to look, in the order a reader would.
//
// The hosts file is included off Windows and not on it. Two measured reasons:
// the Windows one ships with every line commented out and stays that way on an
// ordinary machine -- 21 lines and no entries on the machine this was written
// on -- while `/etc/hosts` on a Unix box usually holds something real. And a
// hosts file is the file most likely to have been replaced wholesale by an
// ad-blocking list of tens of thousands of names, which would bury the handful
// that were meant.
func hostSources(home string) []string {
	sources := []string{filepath.Join(home, ".ssh", "config")}
	if path := hostsFilePath(); path != "" {
		sources = append(sources, path)
	}
	return sources
}

func fingerprintFiles(paths []string) string {
	var fingerprint strings.Builder
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			// An absent file is a state like any other, and it has to be
			// distinguishable from a present one or creating ~/.ssh/config would
			// never be noticed.
			fmt.Fprintf(&fingerprint, "%s|absent\n", path)
			continue
		}
		fmt.Fprintf(&fingerprint, "%s|%d|%d\n", path, info.Size(), info.ModTime().UnixNano())
	}
	return fingerprint.String()
}

func scanHostSources(paths []string) []string {
	seen := map[string]bool{}
	var names []string
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			// A missing ~/.ssh/config is the common case on a fresh machine and
			// is not worth a word to anyone.
			continue
		}
		scan := bufio.NewScanner(file)
		for scan.Scan() {
			for _, name := range hostNamesInLine(scan.Text(), isSSHConfig(path)) {
				add(name)
			}
		}
		file.Close()
	}
	sort.Strings(names)
	return names
}

func isSSHConfig(path string) bool {
	return strings.EqualFold(filepath.Base(path), "config")
}
