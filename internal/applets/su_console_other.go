//go:build !windows

package applets

// No console to hand over here, and nothing that would need one: su is not
// registered off Windows. See su_other.go.
func currentConsole() consoleHandover { return absentConsole{} }

type absentConsole struct{}

func (absentConsole) usable() bool { return false }

func (absentConsole) ownerProcessID() int { return 0 }
