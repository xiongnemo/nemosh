package runtime

import (
	"strings"
	"sync"
)

// What `bind` sets, which the line editor reads keys through: per keymap, the key sequences a
// person bound -- to a readline function, to a macro, to a shell command (-x) -- and the readline
// variables. Shared by every snapshot, as complete's table is, since a session has one line
// editor. The keys the editor has before any bind are in readlineActions; a sequence bound here
// is read first, and one `bind -r` takes away is no key at all.

// BindingKind is what a bound key sequence does.
type BindingKind int

const (
	// BindFunction runs a readline function, Text its name.
	BindFunction BindingKind = iota
	// BindMacro types Text, as though its keys had been pressed.
	BindMacro
	// BindCommand runs Text as a shell command with READLINE_LINE and READLINE_POINT, bind -x.
	BindCommand
	// BindRemoved is a sequence bind -r took away, which then does nothing.
	BindRemoved
)

// KeyBinding is one bound key sequence's action.
type KeyBinding struct {
	Kind BindingKind
	Text string
}

type keyBindings struct {
	mu        sync.Mutex
	keymaps   map[string]map[string]KeyBinding
	variables map[string]string
}

func newKeyBindings() *keyBindings {
	return &keyBindings{keymaps: map[string]map[string]KeyBinding{}, variables: map[string]string{}}
}

// keymapNamed is the keymap a name means -- emacs, vi-command or vi-insert -- with the keys
// a name like emacs-meta puts before each sequence.
func keymapNamed(name string) (keymap, prefix string, ok bool) {
	switch name {
	case "emacs", "emacs-standard":
		return "emacs", "", true
	case "emacs-meta":
		return "emacs", "\x1b", true
	case "emacs-ctlx":
		return "emacs", "\x18", true
	case "vi", "vi-command", "vi-move":
		return "vi-command", "", true
	case "vi-insert":
		return "vi-insert", "", true
	}
	return "", "", false
}

func (b *keyBindings) set(keymap string, keys []byte, binding KeyBinding) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.keymaps[keymap] == nil {
		b.keymaps[keymap] = map[string]KeyBinding{}
	}
	b.keymaps[keymap][string(keys)] = binding
}

// bound is every sequence of keymap with its action, the editor's own keys and then what bind
// set over them; a removed one is left out.
func (b *keyBindings) bound(keymap string) map[string]KeyBinding {
	all := map[string]KeyBinding{}
	if keymap != "vi-command" {
		for _, action := range readlineActions {
			for _, keys := range strings.Fields(action.keys) {
				if parsed, err := parseKeyseq(keys); err == nil {
					all[string(parsed)] = KeyBinding{Kind: BindFunction, Text: action.name}
				}
			}
		}
		for c := byte(' '); c < 0x7f; c++ {
			all[string(c)] = KeyBinding{Kind: BindFunction, Text: "self-insert"}
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for keys, binding := range b.keymaps[keymap] {
		if binding.Kind == BindRemoved {
			delete(all, keys)
			continue
		}
		all[keys] = binding
	}
	return all
}

// MatchKeyBinding is what the line editor reads keys through: the sequence bind set in keymap
// that pending begins with, if one does, and how many bytes it takes; and more when pending
// is also the start of a longer bound sequence, so that the editor waits for the next key
// before it decides. The editor's own keys are its decoder's, and are not answered here.
func (r Runtime) MatchKeyBinding(keymap string, pending []byte) (binding KeyBinding, consumed int, more bool) {
	if r.bindings == nil {
		return KeyBinding{}, 0, false
	}
	r.bindings.mu.Lock()
	defer r.bindings.mu.Unlock()
	for keys, candidate := range r.bindings.keymaps[keymap] {
		switch {
		case len(keys) > len(pending) && keys[:len(pending)] == string(pending):
			more = true
		case len(keys) > consumed && strings.HasPrefix(string(pending), keys):
			binding, consumed = candidate, len(keys)
		}
	}
	return binding, consumed, more
}

// ReadlineVariable is a readline variable's value, as bind set it or as a new shell has it.
func (r Runtime) ReadlineVariable(name string) string {
	if r.bindings != nil {
		r.bindings.mu.Lock()
		value, set := r.bindings.variables[name]
		r.bindings.mu.Unlock()
		if set {
			return value
		}
	}
	return readlineVariableDefault(name)
}

// readlineVariableDefault is a variable's value in a new shell, and empty for one readline has
// not got.
func readlineVariableDefault(name string) string {
	for _, line := range strings.Split(readlineVariableDefaults, "\n") {
		if variable, value, _ := strings.Cut(line, " "); variable == name {
			return value
		}
	}
	return ""
}
