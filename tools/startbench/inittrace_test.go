package main

import (
	"reflect"
	"testing"
)

func TestParseInitTrace_sumsTheAllocationsOfEachPackage(t *testing.T) {
	// Given: what GODEBUG=inittrace=1 writes, among the run's own output
	output := "init internal/bytealg @0 ms, 0 ms clock, 0 bytes, 0 allocs\n" +
		"init github.com/gdamore/encoding @0.99 ms, 1.0 ms clock, 66624 bytes, 1103 allocs\r\n" +
		"nemosh v1.4.36-master-be6d21af0000\n" +
		"init os @0.99 ms, 0 ms clock, 1400 bytes, 29 allocs\n"

	// When
	trace := parseInitTrace(output)

	// Then
	want := initTrace{"internal/bytealg": 0, "github.com/gdamore/encoding": 1103, "os": 29}
	if !reflect.DeepEqual(trace, want) {
		t.Fatalf("parseInitTrace = %v, want %v", trace, want)
	}
	if total := trace.total(); total != 1132 {
		t.Fatalf("total = %d, want 1132", total)
	}
}

func TestParseInitTrace_findsNothingInARunThatTracedNothing(t *testing.T) {
	// When
	trace := parseInitTrace("nemosh v1.4.36\n")

	// Then
	if len(trace) != 0 {
		t.Fatalf("parseInitTrace = %v, want nothing", trace)
	}
}

func TestGrowth_listsThePackagesThatGrewMostFirst(t *testing.T) {
	// Given
	base := initTrace{"a": 10, "b": 5, "c": 7, "gone": 3}
	head := initTrace{"a": 12, "b": 5, "c": 2, "new": 40}

	// When
	grown := growth(base, head)

	// Then: shrinking and unchanged packages are not growth
	want := []packageGrowth{{name: "new", by: 40}, {name: "a", by: 2}}
	if !reflect.DeepEqual(grown, want) {
		t.Fatalf("growth = %v, want %v", grown, want)
	}
}
