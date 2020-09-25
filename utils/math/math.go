package math

// MinUint returns the minimum of a list of uints.
func MinUint(uints ...uint) uint {
	if len(uints) == 0 {
		return 0
	}

	min := uints[0]
	for _, u := range uints {
		if u < min {
			min = u
		}
	}
	return min
}
