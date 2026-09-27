package runtime

import "sort"

// bashHashTable is where an associative array's keys sit in bash's hashlib, which is the
// order they come out in -- `${!m[@]}`, `${m[@]}`, `declare -p` -- and the same order on every
// run, since nothing in it is random: bucket by bucket, a key's bucket being its FNV-1 hash
// modulo the bucket count, and within a bucket the key set last first. The count starts at
// 1024 and is four times as many each time the table holds twice its count, as hash_grow has
// it; the rehash walks the old buckets in order and so reverses each one's chain.
type bashHashTable struct {
	size   uint32
	count  int
	chains map[uint32][]string
	// walk is the keys in order, kept until the table changes.
	walk []string
}

const bashHashBuckets = 1024

// bashHash is bash's hash_string: FNV-1, 32 bits.
func bashHash(key string) uint32 {
	hash := uint32(2166136261)
	for index := 0; index < len(key); index++ {
		hash *= 16777619
		hash ^= uint32(key[index])
	}
	return hash
}

func (t *bashHashTable) bucket(key string) uint32 {
	return bashHash(key) & (t.size - 1)
}

// insert is a key the table has not got, at the head of its bucket's chain.
func (t *bashHashTable) insert(key string) {
	if t.chains == nil {
		t.size, t.chains = bashHashBuckets, map[uint32][]string{}
	}
	if t.count >= int(t.size)*2 {
		t.grow()
	}
	bucket := t.bucket(key)
	t.chains[bucket] = append([]string{key}, t.chains[bucket]...)
	t.count++
	t.walk = nil
}

// grow is hash_grow: four times the buckets, each old chain walked head to tail onto the
// heads of the new ones.
func (t *bashHashTable) grow() {
	old := t.order()
	t.size *= 4
	t.chains = make(map[uint32][]string, len(t.chains))
	for _, key := range old {
		bucket := t.bucket(key)
		t.chains[bucket] = append([]string{key}, t.chains[bucket]...)
	}
}

func (t *bashHashTable) remove(key string) {
	bucket := t.bucket(key)
	chain := t.chains[bucket]
	for index, existing := range chain {
		if existing == key {
			t.chains[bucket] = append(chain[:index:index], chain[index+1:]...)
			break
		}
	}
	if len(t.chains[bucket]) == 0 {
		delete(t.chains, bucket)
	}
	t.count--
	t.walk = nil
}

// order is the keys as bash walks them.
func (t *bashHashTable) order() []string {
	if t.walk != nil || t.count == 0 {
		return t.walk
	}
	buckets := make([]uint32, 0, len(t.chains))
	for bucket := range t.chains {
		buckets = append(buckets, bucket)
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i] < buckets[j] })
	walk := make([]string, 0, t.count)
	for _, bucket := range buckets {
		walk = append(walk, t.chains[bucket]...)
	}
	t.walk = walk
	return walk
}

// rebuild is a table that walks as walk does, with size buckets: a copy's, or a job's.
func (t *bashHashTable) rebuild(size uint32, walk []string) {
	t.size, t.count, t.chains, t.walk = size, 0, map[uint32][]string{}, nil
	if t.size == 0 {
		t.size = bashHashBuckets
	}
	for _, key := range walk {
		bucket := t.bucket(key)
		t.chains[bucket] = append(t.chains[bucket], key)
		t.count++
	}
}
