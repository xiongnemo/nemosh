//go:build !windows

package runtime

// Elevation in this sense is a Windows idea: a program whose manifest demands
// administrator, refused at CreateProcess. Unix has setuid and sudo, and neither
// makes exec fail this way, so there is nothing to recognise here.
func requiresElevation(error) bool { return false }

// elevationIsAWindowsIdea is what the shared test expects of the platform.
const elevationIsAWindowsIdea = false
