package digest

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"math/rand/v2"
	"strings"
	"testing"
)

var pairs = []struct {
	name         string
	ours, theirs func() hash.Hash
	abc          string // the hash of "abc", from the standard that defines it
}{
	{"MD5", MD5, md5.New, "900150983cd24fb0d6963f7d28e17f72"},
	{"SHA1", SHA1, sha1.New, "a9993e364706816aba3e25717850c26c9cd0d89d"},
	{"SHA256", SHA256, sha256.New, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
	{"SHA384", SHA384, sha512.New384, "cb00753f45a35e8bb5a03d699ac65007272c32ab0eded1631a8b605a43ff5bed8086072ba1e7cc2358baeca134c825a7"},
	{"SHA512", SHA512, sha512.New, "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f"},
}

// Each hash of "abc" is the standard's, and the sizes are the hashes'.
func TestDigest_knownAnswers(t *testing.T) {
	for _, pair := range pairs {
		h := pair.ours()
		h.Write([]byte("abc"))
		if got := hex.EncodeToString(h.Sum(nil)); got != pair.abc {
			t.Errorf("%s(abc) = %s, want %s", pair.name, got, pair.abc)
		}
		if theirs := pair.theirs(); h.Size() != theirs.Size() || h.BlockSize() != theirs.BlockSize() {
			t.Errorf("%s is %d and %d, want %d and %d", pair.name, h.Size(), h.BlockSize(), theirs.Size(), theirs.BlockSize())
		}
	}
}

// Every length to past two blocks, written in pieces split at random, hashes as crypto's does; a
// Sum leaves the hash to go on, twice over; and Reset begins it again.
func TestDigest_agreesWithCryptoHoweverItIsWritten(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 2))
	data := make([]byte, 1025)
	for index := range data {
		data[index] = byte(random.Uint32())
	}
	for _, pair := range pairs {
		for length := 0; length <= len(data); length++ {
			ours, theirs := pair.ours(), pair.theirs()
			for rest := data[:length]; len(rest) > 0; {
				piece := min(len(rest), 1+random.IntN(200))
				ours.Write(rest[:piece])
				theirs.Write(rest[:piece])
				if random.IntN(8) == 0 && !bytes.Equal(ours.Sum(nil), theirs.Sum(nil)) {
					t.Fatalf("%s differs part of the way through %d bytes", pair.name, length)
				}
				rest = rest[piece:]
			}
			if got, want := ours.Sum([]byte("prefix")), theirs.Sum([]byte("prefix")); !bytes.Equal(got, want) {
				t.Fatalf("%s of %d bytes: %x, want %x", pair.name, length, got, want)
			}
		}
		h := pair.ours()
		h.Write([]byte("stale"))
		h.Reset()
		h.Write([]byte("abc"))
		if got := hex.EncodeToString(h.Sum(nil)); got != pair.abc {
			t.Errorf("%s after Reset: %s", pair.name, got)
		}
	}
}

// A megabyte at a time, as checksum applets read, takes no more than crypto's hash does.
func BenchmarkDigest_SHA256(b *testing.B) {
	data := []byte(strings.Repeat("x", 1<<20))
	for _, pair := range []struct {
		name string
		new  func() hash.Hash
	}{{"cng", SHA256}, {"crypto", sha256.New}} {
		b.Run(pair.name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				h := pair.new()
				h.Write(data)
				h.Sum(nil)
			}
		})
	}
}
