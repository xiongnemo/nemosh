//go:build windows

package main

import (
	"io"
	"os"

	"github.com/xiongnemo/nemosh/internal/applets"
	"golang.org/x/sys/windows"
)

// The console's modes are the session's between command lines.
//
// A program that changes them and dies leaves them changed. btop4win crashed over ssh on
// 2026-11-27 and left the input raw with mouse and window events on -- 0xb8 where the session
// had 0x1f7 -- and its own screen buffer without a line feed's return, which Windows 10's
// conhost keeps once for the whole console (internal/applets/stty_windows.go has the
// measurement). Every prompt after it began where the last line ended, and the next program
// was given no echo, no lines and no Ctrl-C, so one that waited for input looked hung. cmd.exe
// puts the modes back after every command, and Git for Windows' bash after every program it did
// not build (cleanup_for_non_cygwin_app, msys2-runtime's console.cc), setting the output's
// whatever it reads. So the session takes the modes when a line has been read and puts them
// back before the next prompt; what stty or reset set in between was asked for, and is kept.

// consoleModes is the console's modes as a command line found them.
type consoleModes struct {
	input   *os.File
	in      uint32
	outputs []heldOutput
	stty    uint64
}

type heldOutput struct {
	file *os.File
	mode uint32
}

// holdConsoleModes takes the input's mode, and each output's that is a console; nothing when
// the input is none, and putBack then does nothing.
func holdConsoleModes(input *os.File, outputs ...io.Writer) consoleModes {
	var in uint32
	if input == nil || windows.GetConsoleMode(windows.Handle(input.Fd()), &in) != nil {
		return consoleModes{}
	}
	modes := consoleModes{input: input, in: in}
	modes.stty, _, _, _ = applets.SttySet()
	for _, output := range outputs {
		var mode uint32
		if file, ok := output.(*os.File); ok && windows.GetConsoleMode(windows.Handle(file.Fd()), &mode) == nil {
			modes.outputs = append(modes.outputs, heldOutput{file: file, mode: mode})
		}
	}
	return modes
}

// putBack gives the console the modes it was held with, or the ones stty last set if it has set
// any since. The input's is set where it reads otherwise; each output's is set whatever it reads,
// since setting it is what gives a line feed back the return another buffer took.
func (m consoleModes) putBack() {
	if m.input == nil {
		return
	}
	in, out, stty := m.in, uint32(0), false
	if changes, sttyIn, sttyOut, hasOut := applets.SttySet(); changes != m.stty {
		in, out, stty = sttyIn, sttyOut, hasOut
	}
	var now uint32
	if windows.GetConsoleMode(windows.Handle(m.input.Fd()), &now) == nil && now != in {
		_ = windows.SetConsoleMode(windows.Handle(m.input.Fd()), in)
	}
	for _, output := range m.outputs {
		mode := output.mode
		if stty {
			mode = out
		}
		_ = windows.SetConsoleMode(windows.Handle(output.file.Fd()), mode)
	}
}
