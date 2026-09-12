package applets

import (
	"bytes"
	"fmt"
	"strings"
)

// The boundary between awk and everything it might want to run.
//
// **internal/applets never spawns an OS process.** That rule is written down in
// docs/design/windows-execution-model.md and enforced by a test, and awk does not get an
// exception: a command in `print | cmd`, `cmd | getline` or `system(cmd)` is looked up in
// the applet registry and refused by name otherwise. `xargs` already draws exactly this
// line (xargs.go:139), and awk uses the same wording so that a program which asks for
// something unavailable hears the same answer from both.
//
// So `"sort" | getline` works, because sort is an applet here, and
// `"c:\\tool.exe" | getline` is refused loudly rather than half-working.
//
// **Shell syntax is refused rather than approximated.** `"echo a; echo b"` needs a shell to
// mean what it says, and treating it as `echo` with the three arguments `a;`, `echo` and `b`
// would be a wrong answer wearing the costume of a right one. An unquoted metacharacter is
// therefore an error; a quoted one is an ordinary character, so `grep '$'` still works.

// awkShellMetacharacters are the ones that would need a shell to mean anything.
const awkShellMetacharacters = ";|&<>$`(){}\n"

// awkCommandWords splits a command string the way a shell splits a simple command.
func awkCommandWords(text string) ([]string, error) {
	var words []string
	var current strings.Builder
	started := false
	for index := 0; index < len(text); index++ {
		character := text[index]
		switch character {
		case ' ', '\t':
			if started {
				words = append(words, current.String())
				current.Reset()
				started = false
			}
			continue
		case '\'', '"':
			closing := strings.IndexByte(text[index+1:], character)
			if closing < 0 {
				return nil, fmt.Errorf("unbalanced %c in the command %q", character, text)
			}
			current.WriteString(text[index+1 : index+1+closing])
			index += closing + 1
			started = true
			continue
		}
		if strings.IndexByte(awkShellMetacharacters, character) >= 0 {
			return nil, fmt.Errorf("%q needs a shell for the %q in it, and awk runs applets directly", text, string(character))
		}
		current.WriteByte(character)
		started = true
	}
	if started {
		words = append(words, current.String())
	}
	if len(words) == 0 {
		return nil, fmt.Errorf("the command %q is empty", text)
	}
	return words, nil
}

// runOutputPipe hands a pipe's applet everything the program wrote to it.
func (in *awkInterp) runOutputPipe(stream *awkOutput) error {
	applet, found := DefaultRegistry.Lookup(stream.command[0])
	if !found {
		return commandNotFound(stream.command[0])
	}
	// Flushed first so the applet's output lands after whatever the program has already
	// printed, rather than ahead of it in the buffer.
	if err := in.buffered.Flush(); err != nil {
		return err
	}
	return applet.Run(in.ctx, stream.command[1:], bytes.NewReader(stream.buffer.Bytes()), in.output, in.errors)
}

// runCommandOutput runs an applet with no input and answers what it wrote, which is what
// `cmd | getline` reads from.
func (in *awkInterp) runCommandOutput(command string) (*bytes.Buffer, error) {
	words, err := awkCommandWords(command)
	if err != nil {
		return nil, err
	}
	applet, found := DefaultRegistry.Lookup(words[0])
	if !found {
		return nil, commandNotFound(words[0])
	}
	var out bytes.Buffer
	// A command read by `getline` gets no input of its own: awk's own stdin belongs to
	// the record loop, and handing it over would make `"cat" | getline` eat the records
	// the program was about to read.
	if err := applet.Run(in.ctx, words[1:], bytes.NewReader(nil), &out, in.errors); err != nil {
		if _, carried := StatusCode(err); !carried {
			return nil, err
		}
		// A command that failed still has whatever it managed to write, and awk reports
		// the failure through the status rather than by refusing to read.
	}
	return &out, nil
}

// builtinSystem runs an applet and answers its exit status.
func (in *awkInterp) builtinSystem(node awkBuiltinExpr) (awkValue, error) {
	command, err := in.argText(node, 0)
	if err != nil {
		return awkValue{}, err
	}
	words, err := awkCommandWords(command)
	if err != nil {
		return awkValue{}, err
	}
	applet, found := DefaultRegistry.Lookup(words[0])
	if !found {
		return awkValue{}, commandNotFound(words[0])
	}
	// Everything buffered goes out first: a program that prints and then calls `system`
	// expects the two outputs in the order it wrote them.
	if err := in.buffered.Flush(); err != nil {
		return awkValue{}, err
	}
	runErr := applet.Run(in.ctx, words[1:], bytes.NewReader(nil), in.output, in.errors)
	if err := in.buffered.Flush(); err != nil {
		return awkValue{}, err
	}
	return awkNum(float64(awkCommandStatus(runErr))), nil
}

// awkCommandStatus is an applet's error as the number `system` answers.
func awkCommandStatus(err error) int {
	if err == nil {
		return 0
	}
	if code, carried := StatusCode(err); carried {
		return code
	}
	return 1
}
