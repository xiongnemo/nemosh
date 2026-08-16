//go:build !windows

package runtime

import "github.com/xiongnemo/nemosh/internal/pathmodel"

// Nothing to fill in here. A Unix login sets HOME, USER, LOGNAME and SHELL
// before any shell starts, so a shell that found them missing would be running
// somewhere deliberately bare, and inventing values would hide that rather than
// help.
func loginDefaults(pathmodel.Model) map[string]string { return nil }
