package applets

import "strings"

// GNU's sort -R, which busybox's has not got: see sortRandom.

// compareRandom is -R: by the keys' hashes, and where two keys that differ hash alike, by their
// bytes, so that equal keys, and only they, compare equal and are kept together.
func (spec sortSpec) compareRandom(left, right string) int {
	if left == right {
		return 0
	}
	leftHash, rightHash := sortRandomHash(spec.seed, left), sortRandomHash(spec.seed, right)
	switch {
	case leftHash < rightHash:
		return -1
	case leftHash > rightHash:
		return 1
	}
	return strings.Compare(left, right)
}

// sortRandomHash is FNV-1a from a basis the seed moves, and murmur3's finaliser to spread it, which
// is enough to shuffle and costs no package the binary did not have.
func sortRandomHash(seed uint64, key string) uint64 {
	hash := uint64(14695981039346656037) ^ seed
	for index := 0; index < len(key); index++ {
		hash = (hash ^ uint64(key[index])) * 1099511628211
	}
	hash ^= hash >> 33
	hash *= 0xff51afd7ed558ccd
	hash ^= hash >> 33
	hash *= 0xc4ceb9fe1a85ec53
	return hash ^ hash>>33
}
