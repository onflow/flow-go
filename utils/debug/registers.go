package debug

import (
	"cmp"
	"slices"

	"github.com/onflow/flow-go/model/flow"
)

func CompareRegisterIDs(a flow.RegisterID, b flow.RegisterID) int {
	return cmp.Or(
		cmp.Compare(a.Owner, b.Owner),
		cmp.Compare(a.Key, b.Key),
	)
}

func SortRegisterIDs(registerIDs []flow.RegisterID) {
	slices.SortFunc(registerIDs, func(a, b flow.RegisterID) int {
		return CompareRegisterIDs(a, b)
	})
}

func SortRegisterEntries(registerEntries []flow.RegisterEntry) {
	slices.SortFunc(registerEntries, func(a, b flow.RegisterEntry) int {
		return CompareRegisterIDs(a.Key, b.Key)
	})
}
