package applets

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestReaderFor_keepsAReaderAlreadyMadeForTheCall(t *testing.T) {
	// Given
	ctx := context.Background()
	reader := &ContextReader{Ctx: ctx, Reader: strings.NewReader("x")}

	// When
	got := readerFor(ctx, reader)

	// Then
	if got != reader {
		t.Fatalf("readerFor wrapped a reader made for the same context: %T", got)
	}
}

func TestReaderFor_wrapsAReaderMadeForAnotherContext(t *testing.T) {
	// Given: env's child applet, called in a context env derived
	parent := context.Background()
	child, cancel := context.WithCancel(parent)
	defer cancel()
	reader := &ContextReader{Ctx: parent, Reader: strings.NewReader("x")}

	// When
	got := readerFor(child, reader)

	// Then: the child's cancellation reaches its reads
	if _, wrapped := got.(contextReader); !wrapped {
		t.Fatalf("readerFor kept a reader made for another context: %T", got)
	}
}

// uncomparableContext is a context whose type == panics on.
type uncomparableContext struct {
	context.Context
	tags []string
}

func (uncomparableContext) Deadline() (time.Time, bool) { return time.Time{}, false }

func TestReaderFor_comparesContextsAnUncomparableTypeHolds(t *testing.T) {
	// Given
	a := uncomparableContext{Context: context.Background(), tags: []string{"a"}}
	b := uncomparableContext{Context: context.Background(), tags: []string{"b"}}
	reader := &ContextReader{Ctx: a, Reader: strings.NewReader("x")}

	// When: a bare == of a and b would panic here
	got := readerFor(b, reader)

	// Then
	if _, wrapped := got.(contextReader); !wrapped {
		t.Fatalf("readerFor = %T, want a fresh wrapper", got)
	}
}
