package main

import "github.com/xiongnemo/nemosh/internal/shell/runtime"

// What `bind` set is read before the editor's own keys: a sequence bound to a readline function
// is that function's key, one bound to a macro puts the macro's keys in front of what is still
// to be read, as though they had been typed, and one bind -r took away is no key at all. A
// sequence that is the start of a longer bound one waits for the key after it, as an Escape
// waits for the rest of its sequence. See runtime's readline_bindings.go.

// maxMacroKeys bounds what macros may put back before a key comes of them, so a macro that
// types its own keys ends rather than going round for ever.
const maxMacroKeys = 4096

// keymap is the keymap the editor reads keys in: emacs, or vi's insert or command one.
func (e *lineEditor) keymap() string {
	switch {
	case !e.vi.on:
		return "emacs"
	case e.vi.command:
		return "vi-command"
	}
	return "vi-insert"
}

// boundKey is the key what is pending begins with, when bind bound it: done is false when
// nothing bound matches and the editor's own decoder is to read it, and wait when more input
// is needed to tell.
func (e *lineEditor) boundKey() (decoded key, done, wait bool) {
	if e.bindings == nil || len(e.pending) == 0 {
		return key{}, false, false
	}
	for put := 0; put < maxMacroKeys; {
		binding, consumed, more := e.bindings(e.keymap(), e.pending)
		if more && (consumed == 0 || e.inputWaiting == nil || e.inputWaiting(escapeWait)) {
			return key{}, false, true
		}
		if consumed == 0 {
			return key{}, false, false
		}
		pressed := string(e.pending[:consumed])
		e.pending = e.pending[consumed:]
		switch binding.Kind {
		case runtime.BindFunction:
			if k, ok := functionKey(binding.Text, pressed); ok {
				return k, true, false
			}
			return key{kind: keyUnknown}, true, false
		case runtime.BindMacro:
			e.pending = append([]byte(binding.Text), e.pending...)
			put += len(binding.Text)
		default:
			return key{kind: keyUnknown}, true, false
		}
		if len(e.pending) == 0 {
			return key{}, false, false
		}
	}
	e.pending = e.pending[:0]
	return key{kind: keyUnknown}, true, false
}
