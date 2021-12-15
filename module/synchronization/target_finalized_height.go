// // TODO: lazy computation of median

// func newTargetHeight(initial uint64, windowSize int) *targetHeight {
// 	return &targetHeight{
// 		cachedTarget: initial,
// 		heights:      make([]uint64, 0, windowSize),
// 		windowSize:   windowSize,
// 	}
// }

// func (t *targetHeight) Update(height uint64) {
// 	if len(t.heights) < t.windowSize {
// 		t.heights = append(t.heights, height)
// 	} else {
// 		// TODO: update outdated flag **if** the new value is diff
// 		t.heights[t.oldestIndex] = height
// 		t.oldestIndex = (t.oldestIndex + 1) % t.windowSize
// 	}
// }

// func (t *targetHeight) Get() uint64 {
// 	if t.outdated {
// 		// TODO: compute
// 		t.outdated = false
// 	}

// 	return t.cachedTarget
// }

// func computeMedian(n ...uint64) uint64 {
// 	sort.Slice(n, func(i, j int) bool { return n[i] < n[j] })

// 	m := len(n) / 2

// 	if len(n)%2 == 1 {
// 		return n[m]
// 	}

// 	return n[m-1]
// }