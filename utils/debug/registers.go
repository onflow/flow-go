package debug

import (
	"cmp"
	"slices"

	"github.com/onflow/flow-go/model/flow"
)

func compareRegisterIDs(a flow.RegisterID, b flow.RegisterID) int {
	return cmp.Or(
		cmp.Compare(a.Owner, b.Owner),
		cmp.Compare(a.Key, b.Key),
	)
}

func sortRegisterIDs(registerIDs []flow.RegisterID) {
	slices.SortFunc(registerIDs, func(a, b flow.RegisterID) int {
		return compareRegisterIDs(a, b)
	})
}

func sortRegisterEntries(registerEntries []flow.RegisterEntry) {
	slices.SortFunc(registerEntries, func(a, b flow.RegisterEntry) int {
		return compareRegisterIDs(a.Key, b.Key)
	})
}
