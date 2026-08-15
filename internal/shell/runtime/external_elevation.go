package runtime

// Elevation is a launch this shell will not perform on its own, and the
// diagnostic has to say that rather than leak Go's wording for it.
//
// `WinSAT` came back as `fork/exec C:\Windows\system32\WinSAT.exe: The requested
// operation requires elevation.` -- "fork/exec" is a Go-ism that names no API
// Windows has, and the sentence does not say who is supposed to do anything
// about it.
//
// busybox-w32 takes the other road here: mingw_execve retries with
// ShellExecuteEx and the `runas` verb when CreateProcess returns
// ERROR_ELEVATION_REQUIRED (win32/process.c:560-566, shell_execute at :514).
// Nemosh deliberately does not, for two reasons that are recorded in
// docs/support-matrix.md under Elevation:
//
//  1. ShellExecuteEx cannot pass handles to the child. The elevated process
//     gets a new console, so every redirection and pipe the user wrote is
//     silently ignored -- `WinSAT formal > report.txt` leaves an empty file and
//     puts the output in a window that closes. Silent partial obedience is the
//     thing this shell refuses elsewhere, and it is no better here.
//  2. A consent dialog that appears because a name was typed teaches the reader
//     to click Yes without reading. Elevation should be asked for.
//
// The failure keeps status 126: the command was found and could not be run,
// which is exactly SUSv3's distinction.
func elevationDiagnostic(name string) shellDiagnostic {
	return shellDiagnostic{
		message: "requires administrator, and this shell does not elevate on its own",
		hint: "start an elevated shell and run it there, or launch it through a tool that " +
			"elevates (`gsudo " + name + " ...`). See docs/support-matrix.md, Elevation",
		channel: debugExec,
	}
}
