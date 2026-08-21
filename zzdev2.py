import io

p = 'cmd/nemosh/complete.go'
s = io.open(p, encoding='utf-8').read()

old = '''func completeOperand(workingDirectory, home, command, prefix string) (matches []string, areOptions bool) {
	if strings.HasPrefix(prefix, "-") {
		if options := completeOption(command, prefix); len(options) > 0 {
			return options, true
		}
	}
	return completePathsFrom(workingDirectory, home, prefix, completesDirectoriesOnly(command)), false
}'''
new = '''func completeOperand(where completionPaths, command, prefix string) (matches []string, areOptions bool) {
	if strings.HasPrefix(prefix, "-") {
		if options := completeOption(command, prefix); len(options) > 0 {
			return options, true
		}
	}
	return completePathsIn(where, prefix, completesDirectoriesOnly(command)), false
}

// completionPaths is what the completion needs to know about the shell's own view of paths.
//
// A struct rather than a growing parameter list. It began as one working directory, gained a home
// directory when `~/` turned out not to complete, and gains the devices here; a fourth positional
// string would be the point at which two of them get swapped by mistake.
//
// Every field is a *snapshot* taken from the runtime, refreshed each time the editor draws a
// prompt, because all three can move while a session runs: `cd` moves the directory, HOME can be
// exported to, and whether `/dev` exists at all is a path-model setting.
type completionPaths struct {
	workingDirectory string
	home             string
	// devices are the names under /dev, empty when the path model has it switched off. Taken
	// from the runtime rather than from a list here, so completion cannot offer a device the
	// shell would then refuse to open.
	devices []string
}'''
assert old in s
s = s.replace(old, new, 1)

old = '''func completePathsFrom(workingDirectory, home, prefix string, directoriesOnly bool) []string {'''
new = '''func completePathsIn(where completionPaths, prefix string, directoriesOnly bool) []string {
	if names, ok := completeDevicePath(where, prefix, directoriesOnly); ok {
		return names
	}
	return completePathsFrom(where.workingDirectory, where.home, prefix, directoriesOnly)
}

// completeDevicePath offers the devices when the word being typed is under /dev.
//
// The devices have no host directory to read, so this answers from the names the runtime reported
// rather than from os.ReadDir. Directories-only takes nothing: every device is a character device,
// so `cd /dev/<TAB>` offering one would propose something `cd` cannot do.
func completeDevicePath(where completionPaths, prefix string, directoriesOnly bool) ([]string, bool) {
	if len(where.devices) == 0 || directoriesOnly {
		return nil, strings.HasPrefix(prefix, "/dev/") && directoriesOnly
	}
	stem, found := strings.CutPrefix(prefix, "/dev/")
	if !found {
		return nil, false
	}
	if strings.Contains(stem, "/") {
		// Something under a device, which cannot exist: a device is not a directory.
		return nil, true
	}
	var matches []string
	for _, name := range where.devices {
		if completionMatches(name, stem) {
			matches = append(matches, "/dev/"+name)
		}
	}
	sort.Strings(matches)
	return matches, true
}

func completePathsFrom(workingDirectory, home, prefix string, directoriesOnly bool) []string {'''
assert old in s
s = s.replace(old, new, 1)
io.open(p, 'w', encoding='utf-8', newline='\n').write(s)

p = 'cmd/nemosh/complete_host.go'
s = io.open(p, encoding='utf-8').read()
old = '''	return completeOperand(e.workingDirectory, e.home, commandInProgress(prefix), stem)'''
new = '''	return completeOperand(e.paths(), commandInProgress(prefix), stem)'''
assert old in s
s = s.replace(old, new, 1)
io.open(p, 'w', encoding='utf-8', newline='\n').write(s)

p = 'cmd/nemosh/lineedit.go'
s = io.open(p, encoding='utf-8').read()
old = '''	// home is the native home directory, so `~/` can be completed. Cached beside the working
	// directory and refreshed with it, because the completion functions hold no Runtime to ask.
	home string'''
new = '''	// home is the native home directory, so `~/` can be completed. Cached beside the working
	// directory and refreshed with it, because the completion functions hold no Runtime to ask.
	home string
	// devices are the names under /dev, for the same reason and refreshed at the same time.
	devices []string'''
assert old in s
s = s.replace(old, new, 1)
s = s.rstrip('\n') + '''

// paths is the editor's snapshot of the shell's path view, as the completion wants it.
func (e *lineEditor) paths() completionPaths {
	return completionPaths{
		workingDirectory: e.workingDirectory,
		home:             e.home,
		devices:          e.devices,
	}
}
'''
io.open(p, 'w', encoding='utf-8', newline='\n').write(s)

p = 'cmd/nemosh/session_edited.go'
s = io.open(p, encoding='utf-8').read()
old = '''		editor.home = completionHome(rt)'''
new = '''		editor.home = completionHome(rt)
		editor.devices = completionDevices(rt)'''
assert old in s
s = s.replace(old, new, 1)
io.open(p, 'w', encoding='utf-8', newline='\n').write(s)
print("stage 2 completion written")
