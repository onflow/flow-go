package status_test

import (
	"container/heap"
	"sort"
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization/requester/status"
	"github.com/onflow/flow-go/utils/unittest"
	"pgregory.net/rapid"
)

type rapidHeap struct {
	heap    *status.NotificationHeap
	n       int
	state   map[uint64]map[flow.Identifier]struct{}
	heights []uint64
	entries int
}

func (r *rapidHeap) Init(t *rapid.T) {
	r.n = rapid.IntRange(1, 1000).Draw(t, "n").(int)
	r.heap = &status.NotificationHeap{}
	r.state = make(map[uint64]map[flow.Identifier]struct{})
}

func (r *rapidHeap) Pop(t *rapid.T) {
	if r.heap.Len() <= 0 {
		t.Skip("heap empty")
	}

	entry, ok := heap.Pop(r.heap).(*status.BlockEntry)
	if !ok {
		t.Fatal("heap.Pop() returned non-block entry")
	}

	if _, has := r.state[entry.Height]; !has {
		t.Fatalf("heap.Pop() returned block entry with unexpected height: %+v", entry)
	}

	if _, has := r.state[entry.Height][entry.BlockID]; !has {
		t.Fatalf("heap.Pop() returned block entry with unexpected block ID: %+v", entry)
	}

	if len(r.state[entry.Height]) == 1 {
		delete(r.state, entry.Height)
	} else {
		delete(r.state[entry.Height], entry.BlockID)
	}
	// r.heights is lazily cleaned up in PeekMin

	r.entries--
}

func (r *rapidHeap) Push(t *rapid.T) {
	if r.heap.Len() >= r.n {
		t.Skip("heap full")
	}

	entry := &status.BlockEntry{
		Height:  rapid.Uint64().Draw(t, "height").(uint64),
		BlockID: unittest.IdentifierFixture(),
	}

	heap.Push(r.heap, entry)

	if _, has := r.state[entry.Height]; !has {
		r.state[entry.Height] = make(map[flow.Identifier]struct{})
		r.heights = append(r.heights, entry.Height)
	}
	r.state[entry.Height][entry.BlockID] = struct{}{}
	r.entries++
}

func (r *rapidHeap) PeekMin(t *rapid.T) {
	if r.heap.Len() == 0 {
		t.Skip("heap empty")
	}

	entry := r.heap.PeekMin()
	if entry == nil {
		t.Fatal("heap.PeekMin() returned nil with non-empty heap")
	}

	// sort the heights ascending so we can scan from the beginning for smaller heights
	sort.Slice(r.heights, func(i, j int) bool {
		return r.heights[i] < r.heights[j]
	})

	firstPos := 0
	for _, h := range r.heights {
		// found a larger height, we're done
		if h >= entry.Height {
			break
		}

		// found a stale smaller height to remove
		if _, has := r.state[h]; !has {
			firstPos++
			continue
		}

		// found a valid smaller height
		t.Fatalf("heap.PeekMin() returned block entry with height %d, but heap has lower height %d", entry.Height, h)
	}

	// remove any stale entries
	r.heights = r.heights[firstPos:]
}

func (r *rapidHeap) Check(t *rapid.T) {
	if r.heap.Len() != r.entries {
		t.Fatalf("heap size mismatch: %v vs expected %v", r.heap.Len(), r.entries)
	}
}

func TestNotificationHeap(t *testing.T) {
	rapid.Check(t, rapid.Run(&rapidHeap{}))
}
