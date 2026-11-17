package runtime

import (
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
)

// The job's state crosses to its process as bytes in an encoding of its own. It was JSON, which
// linked encoding/json for this alone, and turned every byte of a value that was not UTF-8 into
// U+FFFD on the way.
//
// One walk reads and writes it: jobStateWalk names every field once, in order, and the same
// code that writes a field reads it back, so the two directions cannot disagree. A version
// byte comes first. A string is its length and its bytes; an integer is a zigzag varint; a
// slice or a map is its count plus one, 0 for nil, so nil and empty arrive apart, and a map's
// keys go in order. Reading, the first error is kept and everything after it reads nothing;
// a count is never more than the bytes left, so a bad input cannot ask for more than it is;
// and bytes after the end are an error too.

const jobStateVersion = 1

type jobCodec struct {
	writing bool
	data    []byte
	err     error
}

func encodeJobState(state jobState) []byte {
	codec := &jobCodec{writing: true, data: []byte{jobStateVersion}}
	jobStateWalk(codec, &state)
	return codec.data
}

func decodeJobState(data []byte) (jobState, error) {
	var state jobState
	if len(data) == 0 || data[0] != jobStateVersion {
		return state, errors.New("job state: not version 1 of its encoding")
	}
	codec := &jobCodec{data: data[1:]}
	jobStateWalk(codec, &state)
	if codec.err == nil && len(codec.data) != 0 {
		codec.err = fmt.Errorf("%d bytes after its end", len(codec.data))
	}
	if codec.err != nil {
		return jobState{}, fmt.Errorf("job state: %w", codec.err)
	}
	return state, nil
}

func (c *jobCodec) fail(err error) {
	if c.err == nil {
		c.err = err
	}
	c.data = nil
}

func (c *jobCodec) uvarint(value *uint64) {
	if c.writing {
		c.data = binary.AppendUvarint(c.data, *value)
		return
	}
	read, size := binary.Uvarint(c.data)
	if size <= 0 {
		c.fail(errors.New("truncated"))
		return
	}
	*value, c.data = read, c.data[size:]
}

func (c *jobCodec) int64(value *int64) {
	if c.writing {
		c.data = binary.AppendVarint(c.data, *value)
		return
	}
	read, size := binary.Varint(c.data)
	if size <= 0 {
		c.fail(errors.New("truncated"))
		return
	}
	*value, c.data = read, c.data[size:]
}

func (c *jobCodec) int(value *int) {
	wide := int64(*value)
	c.int64(&wide)
	*value = int(wide)
}

func (c *jobCodec) uint32(value *uint32) {
	wide := uint64(*value)
	c.uvarint(&wide)
	*value = uint32(wide)
}

func (c *jobCodec) uint16(value *uint16) {
	wide := uint64(*value)
	c.uvarint(&wide)
	*value = uint16(wide)
}

func (c *jobCodec) bool(value *bool) {
	bit := uint64(0)
	if *value {
		bit = 1
	}
	c.uvarint(&bit)
	*value = bit == 1
}

// count writes the length of a slice or map, plus one for one that is not nil, and reads it back
// as that length and whether there is one at all.
func (c *jobCodec) count(length int, present bool) (int, bool) {
	stored := uint64(0)
	if present {
		stored = uint64(length) + 1
	}
	c.uvarint(&stored)
	if !c.writing && stored > uint64(len(c.data))+1 {
		c.fail(fmt.Errorf("a count of %d with %d bytes left", stored-1, len(c.data)))
		return 0, false
	}
	if stored == 0 {
		return 0, false
	}
	return int(stored - 1), true
}

func (c *jobCodec) string(value *string) {
	length, _ := c.count(len(*value), true)
	if c.writing {
		c.data = append(c.data, *value...)
		return
	}
	if c.err != nil {
		return
	}
	*value, c.data = string(c.data[:length]), c.data[length:]
}

// codecSlice reads or writes a slice, each element through element.
func codecSlice[T any](c *jobCodec, slice *[]T, element func(*jobCodec, *T)) {
	length, present := c.count(len(*slice), *slice != nil)
	if !c.writing {
		if c.err != nil || !present {
			return
		}
		*slice = make([]T, length)
	}
	for index := range *slice {
		element(c, &(*slice)[index])
	}
}

// codecMap reads or writes a map keyed by string, its keys in order, each value through value.
func codecMap[T any](c *jobCodec, entries *map[string]T, value func(*jobCodec, *T)) {
	length, present := c.count(len(*entries), *entries != nil)
	if c.writing {
		keys := make([]string, 0, len(*entries))
		for key := range *entries {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			element := (*entries)[key]
			c.string(&key)
			value(c, &element)
		}
		return
	}
	if c.err != nil || !present {
		return
	}
	*entries = make(map[string]T, length)
	for range length {
		var key string
		var element T
		c.string(&key)
		value(c, &element)
		if c.err != nil {
			return
		}
		(*entries)[key] = element
	}
}

func (c *jobCodec) strings(value *[]string) { codecSlice(c, value, (*jobCodec).string) }

func (c *jobCodec) int64s(value *[]int64) { codecSlice(c, value, (*jobCodec).int64) }

func (c *jobCodec) stringMap(value *map[string]string) { codecMap(c, value, (*jobCodec).string) }
