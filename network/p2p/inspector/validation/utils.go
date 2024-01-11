package validation

// duplicateStrTracker is a map of strings to the number of times they have been tracked.
// It is a non-concurrent map, so it should only be used in a single goroutine.
// It is used to track duplicate strings.
type duplicateStrTracker map[string]uint

// track stores the string and returns the number of times it has been tracked and whether it is a duplicate.
// If the string has not been tracked before, it is stored with a count of 1.
// If the string has been tracked before, the count is incremented.
// Args:
//
//	s: the string to track
//
// Returns:
// The number of times this string has been tracked, e.g., 1 if it is the first time, 2 if it is the second time, etc.
func (d duplicateStrTracker) track(s string) uint {
	if _, ok := d[s]; !ok {
		d[s] = 0
	}
	d[s]++

	return d[s]
}
