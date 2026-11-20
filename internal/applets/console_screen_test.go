package applets

import (
	"os"
	"testing"
)

// tcell reads the terminal's TERM from the process's environment when a screen starts
// (tscreen.go:323), and the TERM that counts is the shell's: one the session set aside is not
// the process's either, and one set at the prompt is.
func TestConsoleTerm_isTheShells(t *testing.T) {
	t.Setenv("TERM", "inherited")
	useShellsTerm(hostProcessView{environ: []string{"PATH=x"}})
	if term, set := os.LookupEnv("TERM"); set {
		t.Errorf("TERM is %q with the shell having none", term)
	}
	useShellsTerm(hostProcessView{environ: []string{"TERM=xterm-256color"}})
	if term := os.Getenv("TERM"); term != "xterm-256color" {
		t.Errorf("TERM is %q, want the shell's", term)
	}
}
