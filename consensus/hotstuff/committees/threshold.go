package committees

// WeightThresholdToBuildQC returns the weight that is minimally required for building a QC.
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

// WeightThresholdForHonestMajority returns the weight that is minimally required for reaching honest majority
func WeightThresholdForHonestMajority(totalWeight uint64) uint64 {
	// Given totalWeight, we need the smallest integer t such that totalWeight / 3 < t
	// Formally, the minimally required weight is: Floor(totalWeight/3) + max(1, totalWeight mod 3)
	res := totalWeight / 3 // integer division, includes floor
	divRemainder := totalWeight % 3
	if divRemainder <= 1 {
		res = res + 1
	} else {
		res += divRemainder
	}
	return res
}
