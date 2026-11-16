package xz

// nemosh's: upstream's tests read and write streams checked with SHA-256, so they are given
// crypto's, which a test may link. See README.md.

import "crypto/sha256"

func init() { SHA256New = sha256.New }
