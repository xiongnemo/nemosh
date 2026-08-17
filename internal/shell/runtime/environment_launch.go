package runtime

import "strings"

// The launch boundary speaks the platform's own path language.
//
// Inside, this shell has one spelling for a path -- `/c/Users/nemo` -- and
// everything it says about a directory uses it: `pwd`, `$PWD`, `echo ~`. Outside
// is a native program that has never heard of it. Measured, with HOME exported
// in the shell's spelling:
//
//	busybox ash -c 'cd $HOME; pwd'  ->  C:/c/Users/nemo
//
// busybox read the leading slash as "absolute on the current drive" and glued
// the drive on in front, which is what any native program does with it.
//
// So the two spellings meet here and nowhere else. Before this the environment
// handed out both at once -- PWD as `/c/...` and OLDPWD as `C:/...`, in the same
// block -- which is not a rule, it is the absence of one.
//
// Only the variables this shell sets itself are rewritten. A value the user
// exported travels verbatim, whatever it looks like. That is the whole
// difference from MSYS2, which rewrites anything resembling a path on its way
// out and is regularly wrong about `--prefix=/opt` and about arguments that were
// never paths at all. Guessing is the failure mode; a fixed list is not.
var launchPathVariables = map[string]bool{
	"HOME":   true,
	"PWD":    true,
	"OLDPWD": true,
	"SHELL":  true,
}

// childEnvironment is what a launched program receives.
func (r Runtime) childEnvironment() []string {
	items := r.env.childEnviron(hostEnvironmentPlatform())
	if hostEnvironmentPlatform() != windowsEnvironment {
		return items
	}
	for index, item := range items {
		name, value, found := strings.Cut(item, "=")
		if !found || !launchPathVariables[name] || value == "" {
			continue
		}
		native, ok := r.nativeSpellingOf(value)
		if !ok {
			continue
		}
		items[index] = name + "=" + native
	}
	return items
}

// nativeSpellingOf converts one value, reporting whether it could.
//
// A value that does not resolve is passed through untouched rather than dropped
// or reported. This runs on every launch, there is nothing useful to say about
// somebody's `PWD=whatever`, and refusing to start a program over the shape of
// an environment variable would be a much worse trade than handing it a string
// it may not want.
func (r Runtime) nativeSpellingOf(value string) (string, bool) {
	resolved, err := r.ResolveNemoshPath(value)
	if err != nil || resolved.Device || resolved.Native == "" {
		return "", false
	}
	return resolved.Native, true
}
