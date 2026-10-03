package runtime_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// _get_comp_words_by_ref reads the line as bash-completion's _comp_get_words does: COMP_WORDS
// with -n's characters no word break, so `--repo=cli/c` is the word it is on the line, and cur,
// prev, words and cword set in the calling function's own variables. cobra's scripts -- gh's,
// kubectl's -- call it when the package's _init_completion is not there, which on Windows it
// seldom is, and stopped there when it was not found either.
func TestCompletionHelpers_getCompWordsByRefJoinsWhatItIsToldTo(t *testing.T) {
	rt := completingShell(t, `_cmd() {
  local cur prev words cword
  _get_comp_words_by_ref -n "$exclude" cur prev words cword
  COMPREPLY=("cur=$cur" "prev=$prev" "cword=$cword" "words=${words[*]}")
}
complete -F _cmd cmd
exclude='=:'
`)
	answer, _ := completeAt(rt, "cmd sub --repo=cli/c|")
	if want := []string{"cur=--repo=cli/c", "prev=sub", "cword=2", "words=cmd sub --repo=cli/c"}; !slices.Equal(answer.Candidates, want) {
		t.Errorf("with -n =: got %q, want %q", answer.Candidates, want)
	}
	rt = completingShell(t, "_cmd() { local cur prev words cword; _get_comp_words_by_ref cur prev words cword; COMPREPLY=(\"cur=$cur\" \"prev=$prev\" \"cword=$cword\"); }\ncomplete -F _cmd cmd\n")
	answer, _ = completeAt(rt, "cmd sub --repo=cli/c|")
	if want := []string{"cur=cli/c", "prev==", "cword=4"}; !slices.Equal(answer.Candidates, want) {
		t.Errorf("without -n: got %q, want %q", answer.Candidates, want)
	}
}

// _filedir adds the files the word begins -- with an extension, those ending in it and the
// directories -- and marks the completion's answer as names; _init_completion sets the four
// variables and completes a file itself after a redirection; __ltrim_colon_completions takes
// the word up to its colon off each answer, since the editor replaces only what follows it.
func TestCompletionHelpers_filedirInitCompletionAndColons(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"a.txt", "b.log"} {
		if err := os.WriteFile(filepath.Join(directory, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(directory, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	rt := completingShell(t, "cd '"+filepath.ToSlash(directory)+`'
_txt() { local cur prev words cword; _init_completion || return; _filedir txt; }
complete -F _txt t
_colon() { local cur; _get_comp_words_by_ref -n : cur; COMPREPLY=(a:bc a:bd); __ltrim_colon_completions "$cur"; }
complete -F _colon c
`)
	answer, _ := completeAt(rt, "t |")
	if want := []string{"a.txt", "dir"}; !slices.Equal(answer.Candidates, want) || !answer.Options["filenames"] {
		t.Errorf("_filedir txt: got %q, %v; want %q and filenames", answer.Candidates, answer.Options, want)
	}
	answer, _ = completeAt(rt, "t x > b|")
	if want := []string{"b.log"}; !slices.Equal(answer.Candidates, want) {
		t.Errorf("after a redirection: got %q, want %q", answer.Candidates, want)
	}
	answer, _ = completeAt(rt, "c a:b|")
	if want := []string{"bc", "bd"}; !slices.Equal(answer.Candidates, want) {
		t.Errorf("__ltrim_colon_completions: got %q, want %q", answer.Candidates, want)
	}
}
