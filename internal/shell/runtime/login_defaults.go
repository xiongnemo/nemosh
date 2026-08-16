package runtime

// Windows does not set the variables a shell assumes it was logged in with.
//
// A session started from Git Bash inherits HOME, USER, LOGNAME and SHELL from
// the environment that launched it, and everything works. A session started
// straight from Windows Terminal inherits none of them, and the consequences are
// not obviously connected to each other:
//
//   - `echo ~` prints a literal tilde, because expansion reads $HOME.
//   - `cd` with no operand answers "cd: HOME not set".
//   - No history is saved, because HISTFILE has nowhere to default to.
//   - `ssh ` completes no host, because ~/.ssh/config cannot be found.
//   - A busybox ash started inside cannot source ~/.profile.
//
// One cause, five symptoms, and the shell looked half-broken in a way that
// pointed at each feature rather than at the variable.
//
// busybox-w32 does the same thing at init and for the same reason: it fills in
// USER, LOGNAME, HOME and SHELL through setvar_if_unset when the environment
// suggests it was not launched from another busybox (shell/ash.c:16471-16478,
// helper at :16388). The home directory it uses comes from
// GetUserProfileDirectory on the process token rather than from %USERPROFILE%
// (win32/mingw.c, gethomedir).
//
// Only if unset. An empty HOME is treated as unset rather than honoured, which
// differs from busybox and is the more useful reading here: nothing in this
// shell can do anything with an empty home, and a Windows environment that
// carries `HOME=` is far more likely to have inherited an accident than to be
// asking for one.
func (r Runtime) applyLoginDefaults() {
	for name, value := range loginDefaults(r.paths.model) {
		if value == "" || r.vars[name] != "" {
			continue
		}
		r.vars[name] = value
		// Exported, not just set: the point of HOME is that the ash or the ssh
		// started from here finds it too.
		r.env.Set(name, value)
	}
}
