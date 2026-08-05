package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestRuntime_snapshotClonesMappingsAndSharesDescriptions(t *testing.T) {
	resource := &countingReadWriteCloser{}
	parent := New(applets.DefaultRegistry, Streams{})
	if err := parent.fds.bindOwned(49, resource, readWrite); err != nil {
		t.Fatalf("bind owned: %v", err)
	}
	child, err := parent.snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if err := child.fds.close(49); err != nil {
		t.Fatalf("close child mapping: %v", err)
	}
	if _, err := parent.fds.writer(49); err != nil {
		t.Fatalf("parent mapping changed: %v", err)
	}
	if resource.closes != 0 {
		t.Fatalf("shared resource closed early: %d", resource.closes)
	}
	if err := child.fds.closeAll(); err != nil {
		t.Fatalf("release child: %v", err)
	}
	if err := parent.fds.closeAll(); err != nil {
		t.Fatalf("release parent: %v", err)
	}
	if resource.closes != 1 {
		t.Fatalf("owned close count: %d", resource.closes)
	}
}

func TestRuntime_snapshotReleaseWorksInBothOrders(t *testing.T) {
	for _, childFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "child first", false: "parent first"}[childFirst], func(t *testing.T) {
			resource := &countingReadWriteCloser{}
			parent := New(applets.DefaultRegistry, Streams{})
			if err := parent.fds.bindOwned(3, resource, readWrite); err != nil {
				t.Fatalf("bind: %v", err)
			}
			child, err := parent.snapshot(context.Background())
			if err != nil {
				t.Fatalf("snapshot: %v", err)
			}
			first, second := parent.fds, child.fds
			if childFirst {
				first, second = child.fds, parent.fds
			}
			if err := first.closeAll(); err != nil {
				t.Fatalf("first close: %v", err)
			}
			if resource.closes != 0 {
				t.Fatalf("closed before last owner: %d", resource.closes)
			}
			if err := second.closeAll(); err != nil {
				t.Fatalf("second close: %v", err)
			}
			if resource.closes != 1 {
				t.Fatalf("final close count: %d", resource.closes)
			}
		})
	}
}

func TestFDTable_aliasSharesOffsetAndSurvivesSourceRebind(t *testing.T) {
	source := bytes.NewBufferString("abcdef")
	table := newFDTable(Streams{})
	if err := table.bindBorrowedReader(3, source); err != nil {
		t.Fatalf("bind source: %v", err)
	}
	if err := table.alias(49, 3, readable); err != nil {
		t.Fatalf("alias: %v", err)
	}
	if err := table.bindBorrowedReader(3, bytes.NewBufferString("new")); err != nil {
		t.Fatalf("rebind source: %v", err)
	}

	reader, err := table.reader(49)
	if err != nil {
		t.Fatalf("alias reader: %v", err)
	}
	data := make([]byte, 3)
	if _, err := reader.Read(data); err != nil {
		t.Fatalf("read alias: %v", err)
	}
	if string(data) != "abc" {
		t.Fatalf("alias data: %q", data)
	}
}
