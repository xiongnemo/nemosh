//go:build !windows

package proc

// The per-process extras are a Windows concern: elsewhere the command line and the owner come
// from the same `/proc` read as everything else, and there is nothing to open and nothing to be
// refused. Sampling is not implemented off Windows at all, so nothing here is ever consulted.

// Details is what could be learned about a process beyond the table.
type Details struct {
	Path        string
	CommandLine string
	User        string
}

// Command is the best available description of what is running.
func (d Details) Command(name string) string {
	if d.CommandLine != "" {
		return d.CommandLine
	}
	return name
}

// DetailCache remembers what was learned. Empty everywhere but Windows.
type DetailCache struct{}

// NewDetailCache returns an empty cache.
func NewDetailCache() *DetailCache { return &DetailCache{} }

// Lookup knows nothing here, and an empty Details is how that is said.
func (c *DetailCache) Lookup(process Process) Details { return Details{} }

// Forget has nothing to forget.
func (c *DetailCache) Forget(live map[int]bool) {}
