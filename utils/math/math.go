package math

// MinUint returns the minimum of a list of uints.
func MinUint(uints ...uint) uint {

	min := uint(0)
	for _, u := range uints {
		if u < min {
			min = u
		}
	}
	return min
}
