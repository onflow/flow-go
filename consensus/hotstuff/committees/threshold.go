package committees

// WeightThresholdToBuildQC returns the weight (sum of unique, valid votes for this view)
// that is minimally required for building a QC.
func WeightThresholdToBuildQC(totalWeight uint64) uint64 {
	// Given totalWeight, we need the smallest integer t such that 2 * totalWeight / 3 < t
	// Formally, the minimally required weight is: 2 * Floor(totalWeight/3) + max(1, totalWeight mod 3)
	floorOneThird := totalWeight / 3 // integer division, includes floor
	res := 2 * floorOneThird
	divRemainder := totalWeight % 3
	if divRemainder <= 1 {
		res = res + 1
	} else {
		res += divRemainder
	}
	return res
}

// WeightThresholdToTimeout returns the weight (sum of unique, valid timeout objects for this view)
// that is minimally required to immediately timeout and build a TO.
func WeightThresholdToTimeout(totalWeight uint64) uint64 {
	// Given totalWeight, we need the smallest integer t such that totalWeight / 3 < t
	// Formally, the minimally required weight is: Floor(totalWeight/3) + 1
	return totalWeight/3 + 1
}
