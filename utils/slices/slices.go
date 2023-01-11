package slices

import "sort"

// Concat concatenates multiple []byte into one []byte with efficient one-time allocation.
func Concat(slices [][]byte) []byte {
	var totalLen int
	for _, s := range slices {
		totalLen += len(s)
	}
	tmp := make([]byte, totalLen)
	var i int
	for _, s := range slices {
		i += copy(tmp[i:], s)
	}
	return tmp
}

// EnsureByteSliceSize returns a copy of input bytes with given length
// trimming or left-padding with zero bytes accordingly
func EnsureByteSliceSize(b []byte, length int) []byte {
	if len(b) > length {
		b = b[len(b)-length:]
	}
	var stateBytes = make([]byte, length)
	copy(stateBytes[length-len(b):], b)

	return stateBytes
}

// MakeRange returns a slice of int from [min, max]
func MakeRange(min, max int) []int {
	a := make([]int, max-min+1)
	for i := range a {
		a[i] = min + i
	}
	return a
}

// AreStringSlicesEqual returns true if the two string slices are equal.
func AreStringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	sort.Strings(a)
	sort.Strings(b)

	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// StringSliceContainsElement returns true if the string slice contains the element.
func StringSliceContainsElement(a []string, v string) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}

	return false
}
