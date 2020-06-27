package slices

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
// trimming or padding with zeros front bytes accordingly
func EnsureByteSliceSize(b []byte, length int) []byte {
	if len(b) > length {
		b = b[len(b)-length:]
	}
	var stateBytes = make([]byte, length)
	copy(stateBytes[length-len(b):], b)

	return stateBytes
}
