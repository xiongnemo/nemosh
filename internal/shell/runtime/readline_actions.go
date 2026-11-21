package runtime

import "strings"

// readlineActions are the readline functions this shell's line editor carries out, each with
// the keys it is on in emacs mode, and in vi's insert mode, before any bind -- as bash's bind -p
// names them. cmd/nemosh maps each name to what the editor does, and a test there decodes each
// key and finds that action, so this list cannot drift from the editor. ^D is left out: the
// editor ends input on it, where readline's delete-char ends input only on an empty line. A key
// bound to a function readline has and this list has not is refused, since it would do nothing.
var readlineActions = [...]struct{ name, keys string }{
	{"abort", `\C-g`},
	{"accept-line", `\C-j \C-m`},
	{"backward-char", `\C-b \e[D`},
	{"backward-delete-char", `\C-?`},
	{"backward-kill-word", `\e\C-? \e\C-h`},
	{"backward-word", `\eb`},
	{"beginning-of-line", `\C-a \e[H \e[1~`},
	{"capitalize-word", `\ec`},
	{"clear-screen", `\C-l`},
	{"complete", `\C-i`},
	{"delete-char", `\e[3~`},
	{"downcase-word", `\el`},
	{"edit-and-execute-command", `\C-x\C-e`},
	{"emacs-editing-mode", ""},
	{"exchange-point-and-mark", `\C-x\C-x`},
	{"end-of-line", `\C-e \e[F \e[4~`},
	{"forward-char", `\C-f \e[C`},
	{"forward-word", `\ef`},
	{"history-search-backward", `\e[5~`},
	{"history-search-forward", `\e[6~`},
	{"kill-line", `\C-k`},
	{"kill-word", `\ed`},
	{"next-history", `\C-n \e[B`},
	{"previous-history", `\C-p \e[A`},
	{"redraw-current-line", ""},
	{"reverse-search-history", `\C-r`},
	{"revert-line", `\er`},
	// self-insert is every printable character, which bound lists rather than this.
	{"self-insert", ""},
	{"set-mark", `\C-@ \e\x20`},
	{"shell-expand-line", `\e\C-e`},
	{"transpose-chars", `\C-t`},
	{"undo", `\C-_ \C-x\C-u`},
	{"unix-line-discard", `\C-u`},
	{"unix-word-rubout", `\C-w`},
	{"upcase-word", `\eu`},
	{"vi-editing-mode", `\e\C-j`},
	{"vi-insertion-mode", ""},
	{"vi-movement-mode", ""},
	{"yank", `\C-y`},
	{"yank-last-arg", `\e. \e_`},
	{"yank-pop", `\ey`},
}

// ReadlineActionNames are the functions the line editor carries out, for cmd/nemosh's test that
// maps each one.
func ReadlineActionNames() []string {
	names := make([]string, len(readlineActions))
	for index, action := range readlineActions {
		names[index] = action.name
	}
	return names
}

// ReadlineActionKeys are the keys a function is on before any bind, in readline's notation.
func ReadlineActionKeys(name string) []string {
	for _, action := range readlineActions {
		if action.name == name {
			return strings.Fields(action.keys)
		}
	}
	return nil
}

// ParseKeyseq is the bytes a key sequence in readline's notation stands for, for cmd/nemosh's
// test that decodes each default key.
func ParseKeyseq(text string) ([]byte, error) { return parseKeyseq(text) }

// readlineCarriesOut is whether the line editor does what name asks.
func readlineCarriesOut(name string) bool {
	for _, action := range readlineActions {
		if action.name == name {
			return true
		}
	}
	return false
}

// readlineHas is whether name is a function readline has, carried out here or not.
func readlineHas(name string) bool {
	for _, known := range strings.Fields(readlineFunctionNames) {
		if known == name {
			return true
		}
	}
	return false
}
