package applets

import (
	"reflect"
	"testing"
)

func TestParseCutRanges_mergesLaterRanges_whenOpenRangeAlreadyCoversThem(t *testing.T) {
	// Given
	list := "2-,5,9-10"

	// When
	got, err := parseCutRanges(list)

	// Then
	if err != nil {
		t.Fatalf("expected range parsing to succeed, got %v", err)
	}
	want := []cutRange{{start: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected merged ranges %#v, got %#v", want, got)
	}
}
