//go:build windows

package main

// The Windows hosts file is not read.
//
// It exists -- %SystemRoot%\System32\drivers\etc\hosts -- and on an ordinary
// machine it is the file Microsoft shipped with every line commented out.
// Measured on the machine this was written on: 21 lines, no entries at all. What
// it is *often* replaced with is an ad-blocking list of tens of thousands of
// names, and completing those would bury the handful anyone meant.
//
// So the cost is a file that usually says nothing and occasionally says far too
// much. Off here; on everywhere else, where /etc/hosts is small and curated.
func hostsFilePath() string { return "" }
