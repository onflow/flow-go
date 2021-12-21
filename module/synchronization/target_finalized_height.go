package synchronization

import (
	"math/rand"

	"github.com/onflow/flow-go/model/flow"
)

type TargetHeight struct {
	oldestIndex  int
	windowSize   int
	heights      []uint64
	updated      bool
	cachedMedian uint64
}

func (t *TargetHeight) Update(height uint64, originID flow.Identifier) {
	if len(t.heights) < t.windowSize {
		t.heights = append(t.heights, height)
	} else {
		if t.heights[t.oldestIndex] != height {
			t.heights[t.oldestIndex] = height
			t.updated = true
		}

		t.oldestIndex = (t.oldestIndex + 1) % t.windowSize
	}
}

func (t *TargetHeight) Get() uint64 {
	if t.updated {
		t.cachedMedian = median(t.heights)
		t.updated = false
	}

	return t.cachedMedian
}

func median(l []uint64) uint64 {
	return quickSelect(l, len(l)/2)
}

func quickSelect(l []uint64, k int) uint64 {
	if len(l) == 1 {
		return l[0]
	}

	pivot := l[rand.Intn(len(l))]

	var left []uint64
	var right []uint64
	num_pivots := 0

	for _, v := range l {
		if v < pivot {
			left = append(left, v)
		} else if v > pivot {
			right = append(right, v)
		} else {
			num_pivots++
		}
	}

	if k < len(left) {
		return quickSelect(left, k)
	} else if k < len(left)+num_pivots {
		return pivot
	} else {
		return quickSelect(right, k-len(left)-num_pivots)
	}
}
