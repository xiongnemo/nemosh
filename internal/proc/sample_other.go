//go:build !windows

package proc

// Sampling is implemented on Windows only, for the reason the package comment gives about
// listing: Linux here is a build-and-test target rather than a supported one, and a monitor built
// on `/proc` would be a second implementation of everything in this package for a platform that
// already has htop.
//
// Refused rather than answered emptily, which is the rule the whole package follows: "nothing is
// running" and "I cannot see" are different answers and only one of them is safe to act on.

// Sampler exists on every platform so a caller compiles everywhere; only Windows can fill it.
type Sampler struct{}

// NewSampler returns a sampler that will refuse.
func NewSampler() *Sampler { return &Sampler{} }

// Sample reports that this platform has no implementation.
func (s *Sampler) Sample(withThreads bool) (Snapshot, error) { return Snapshot{}, ErrListUnsupported }
