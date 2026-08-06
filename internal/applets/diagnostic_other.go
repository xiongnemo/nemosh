//go:build !windows

package applets

// Only Windows needs an errno override; elsewhere the portable fs sentinels and
// the underlying syscall error text already agree with strerror.
func platformCauseText(error) (string, bool) { return "", false }
