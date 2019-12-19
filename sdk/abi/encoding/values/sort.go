package values

import "sort"

// SortInEncodingOrder is a central point to keep ordering for encoding the same.
func SortInEncodingOrder(names []string) {
	sort.Strings(names)
}
