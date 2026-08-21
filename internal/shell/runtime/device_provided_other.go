//go:build !windows

package runtime

// runtimeProvidesDev says whether this shell invents `/dev`.
//
// False here: these systems have a real one with the machine's devices in it, and shadowing that
// with eight synthetic names would hide the hardware. See path_state_other.go, where the decision
// lives.
const runtimeProvidesDev = false
