package runtime

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// fillEveryField sets every field of value, however deep, to something that is not its zero
// value, each different from the last: a field added to jobState and not to its codec comes
// back zero and fails the round trip.
func fillEveryField(value reflect.Value, counter *int) {
	*counter++
	switch value.Kind() {
	case reflect.String:
		value.SetString(fmt.Sprintf("s%d\xff\x00é", *counter))
	case reflect.Int:
		value.SetInt(int64(-*counter * 1000003))
	case reflect.Uint16, reflect.Uint32:
		value.SetUint(uint64(*counter))
	case reflect.Bool:
		value.SetBool(true)
	case reflect.Array:
		for index := range value.Len() {
			fillEveryField(value.Index(index), counter)
		}
	case reflect.Slice:
		value.Set(reflect.MakeSlice(value.Type(), 2, 2))
		for index := range 2 {
			fillEveryField(value.Index(index), counter)
		}
	case reflect.Map:
		value.Set(reflect.MakeMap(value.Type()))
		for range 2 {
			key, element := reflect.New(value.Type().Key()).Elem(), reflect.New(value.Type().Elem()).Elem()
			fillEveryField(key, counter)
			fillEveryField(element, counter)
			value.SetMapIndex(key, element)
		}
	case reflect.Struct:
		for index := range value.NumField() {
			fillEveryField(value.Field(index), counter)
		}
	default:
		panic("fillEveryField has no case for " + value.Kind().String())
	}
}

// The job's state crosses to its process in a codec of its own rather than JSON, which linked
// encoding/json for this alone. Every field comes back as it went, a string that is not UTF-8
// as it was -- JSON made each bad byte U+FFFD -- and nil apart from empty, which AliasView
// depends on.
func TestJobStateCodec_roundTripsEveryField(t *testing.T) {
	var state jobState
	counter := 0
	fillEveryField(reflect.ValueOf(&state).Elem(), &counter)
	for name, nilState := range map[string]jobState{
		"every field": state,
		"nil and empty apart": {AliasView: map[string]string{}, Readonly: []string{}, Vars: nil,
			Indexed: map[string]jobIndexedArray{"a": {Values: []string{}, Live: nil}}},
		"nothing": {},
	} {
		decoded, err := decodeJobState(encodeJobState(nilState))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !reflect.DeepEqual(decoded, nilState) {
			t.Errorf("%s came back as\n%#v\nwant\n%#v", name, decoded, nilState)
		}
	}
}

// What did not come from encodeJobState is refused, with a reason and no panic: every prefix
// of a real encoding, another version's, and one with bytes after its end.
func TestJobStateCodec_refusesWhatItDidNotWrite(t *testing.T) {
	var state jobState
	counter := 0
	fillEveryField(reflect.ValueOf(&state).Elem(), &counter)
	encoded := encodeJobState(state)
	for length := range len(encoded) {
		if _, err := decodeJobState(encoded[:length]); err == nil {
			t.Fatalf("the first %d of %d bytes decoded", length, len(encoded))
		}
	}
	other := append([]byte{encoded[0] + 1}, encoded[1:]...)
	if _, err := decodeJobState(other); err == nil || !strings.Contains(err.Error(), "version") {
		t.Errorf("another version decoded: %v", err)
	}
	if _, err := decodeJobState(append(encoded, 0)); err == nil {
		t.Error("a byte after the end decoded")
	}
	// A count no input could hold is refused before anything is made for it.
	huge := append([]byte{encoded[0]}, 0xff, 0xff, 0xff, 0xff, 0x0f)
	if _, err := decodeJobState(huge); err == nil {
		t.Error("a count larger than the input decoded")
	}
}

// Seeds only: fuzzing runs on CI, never on the machine that develops this.
func FuzzDecodeJobState(f *testing.F) {
	var state jobState
	counter := 0
	fillEveryField(reflect.ValueOf(&state).Elem(), &counter)
	f.Add(encodeJobState(state))
	f.Add(encodeJobState(jobState{}))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		decoded, err := decodeJobState(data)
		if err != nil {
			return
		}
		again, err := decodeJobState(encodeJobState(decoded))
		if err != nil || !reflect.DeepEqual(again, decoded) {
			t.Fatalf("a decoded state did not survive a second round trip: %v", err)
		}
	})
}
