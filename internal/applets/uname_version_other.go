//go:build !windows

package applets

// Off Windows the fields exist but Go exposes no portable way to read them, and
// this platform is a build-and-test target rather than a supported one
// (docs/support-matrix.md). Saying unknown is better than inventing a number.
func osReleaseAndVersion() (string, string) { return unameUnknown, unameUnknown }
