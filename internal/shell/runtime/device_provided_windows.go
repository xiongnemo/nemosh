//go:build windows

package runtime

// runtimeProvidesDev says whether this shell invents `/dev`.
//
// True here because Windows has none. The constant exists so that a *shared* test can assert the
// right half without duplicating itself: see path_state_test.go, which resolves `/dev/null` on both
// platforms and has to expect a different answer on each.
const runtimeProvidesDev = true
