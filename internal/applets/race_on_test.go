//go:build race

package applets

// raceDetector says the tests were built with -race, under which sync.Pool drops one Put in
// four on purpose, so a count of allocations through a pool measures the detector, not the code.
const raceDetector = true
