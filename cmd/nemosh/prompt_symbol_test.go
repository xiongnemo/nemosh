package main

import "github.com/xiongnemo/nemosh/internal/applets"

// promptSymbol is what the default prompt's \$ draws for the account the tests run as: # when
// `id -u` says 0, which on Windows is an elevated shell, as busybox-w32 has it.
func promptSymbol() string {
	if applets.CurrentUserID() == 0 {
		return "#"
	}
	return "$"
}
