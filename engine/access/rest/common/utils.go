package common

// SliceToMap converts a slice of strings into a map where each string
// in the slice becomes a key in the map with the value set to true.
func SliceToMap(values []string) map[string]bool {
	valueMap := make(map[string]bool, len(values))
	for _, v := range values {
		valueMap[v] = true
	}
	return valueMap
}
