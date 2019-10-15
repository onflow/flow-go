// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package overlay

import (
	"encoding/hex"
)

// FilterFunc is a function to filter peers by the information they hold.
type FilterFunc func(*Info) bool

// HasNotSeen is a filter that selects peers only when they have not seen the
// event with the given id yet.
func HasNotSeen(ids ...[]byte) FilterFunc {
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		key := hex.EncodeToString(id)
		keys = append(keys, key)
	}
	return func(i *Info) bool {
		for _, key := range keys {
			_, ok := i.Seen[key]
			if ok {
				return false
			}
		}
		return true
	}
}
