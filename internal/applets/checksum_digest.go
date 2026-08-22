package applets

import (
	"hash/crc32"
	"io"
)

// The four non-cryptographic checksums, accumulated in one pass over the input.
//
// One pass rather than four, because a tool that read the file once per algorithm
// would be four times as slow for no reason -- and these are the tools reached
// for on a large download.

// sizedDigest is the running state. Every one of these needs the byte count as
// well as the bytes, which no cryptographic hash here does.
type sizedDigest struct {
	size int64
	// ieee is the ordinary reflected CRC-32, which `crc32` prints.
	ieee uint32
	// posix is cksum's register: the same width but a different polynomial,
	// walked most-significant-bit first and not reflected.
	posix uint32
	// bsd is the rotating 16-bit accumulator; systemV is a plain byte total,
	// folded at the end.
	bsd     uint32
	systemV uint64
}

func readSizedDigest(input io.Reader) (sizedDigest, error) {
	digest := sizedDigest{ieee: crc32.NewIEEE().Sum32()}
	table := crc32.MakeTable(crc32.IEEE)
	ieee := uint32(0)
	buffer := make([]byte, 64*1024)
	for {
		read, err := input.Read(buffer)
		if read > 0 {
			chunk := buffer[:read]
			digest.size += int64(read)
			ieee = crc32.Update(ieee, table, chunk)
			for _, b := range chunk {
				digest.posix = digest.posix<<8 ^ cksumTable[byte(digest.posix>>24)^b]
				// BSD rotates right by one before adding, so a byte's influence
				// moves through the whole register rather than staying in the
				// low bits. That rotation is the only thing separating this from
				// the System V sum below.
				digest.bsd = digest.bsd>>1 | (digest.bsd&1)<<15
				digest.bsd = (digest.bsd + uint32(b)) & 0xFFFF
				digest.systemV += uint64(b)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return sizedDigest{}, err
		}
	}
	digest.ieee = ieee
	return digest, nil
}

func (d sizedDigest) ieeeCRC() uint32 { return d.ieee }

// posixCRC finishes cksum's register by feeding the file length through it and
// complementing the result.
//
// The length is what makes cksum stronger than a bare CRC: two files that differ
// only in trailing zero bytes hash the same without it. The bytes go in
// low-order first and stop as soon as the remaining length is zero, so an empty
// file contributes none -- which is why `cksum` on an empty file answers
// 4294967295, the complement of an untouched register. That value is the
// clearest single check that this is implemented correctly.
func (d sizedDigest) posixCRC() uint32 {
	crc := d.posix
	for length := d.size; length != 0; length >>= 8 {
		crc = crc<<8 ^ cksumTable[byte(crc>>24)^byte(length)]
	}
	return ^crc
}

func (d sizedDigest) bsdSum() uint32 { return d.bsd }

// systemVSum folds the running byte total into sixteen bits.
//
// Twice, not once: the first fold can itself carry out of sixteen bits, and
// dropping that carry is a real difference for inputs over about a megabyte.
func (d sizedDigest) systemVSum() uint64 {
	folded := (d.systemV & 0xFFFF) + (d.systemV >> 16)
	return (folded & 0xFFFF) + (folded >> 16)
}

// cksumTable is the CRC-32 table POSIX specifies for cksum: polynomial
// 0x04C11DB7 walked most-significant-bit first.
//
// It cannot come from hash/crc32, whose tables are all reflected -- Go builds
// them least-significant-bit first, which is the right convention for the IEEE
// CRC every network protocol uses and the wrong one here. Feeding cksum's
// polynomial to crc32.MakeTable produces a plausible number that is not cksum.
var cksumTable = buildCksumTable()

func buildCksumTable() [256]uint32 {
	const polynomial = 0x04C11DB7
	var table [256]uint32
	for index := range table {
		crc := uint32(index) << 24
		for range 8 {
			if crc&0x80000000 != 0 {
				crc = crc<<1 ^ polynomial
			} else {
				crc <<= 1
			}
		}
		table[index] = crc
	}
	return table
}
