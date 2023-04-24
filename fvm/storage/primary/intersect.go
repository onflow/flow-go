package primary

import (
	"github.com/onflow/flow-go/model/flow"
)

func intersectHelper[
	T1 any,
	T2 any,
](
	smallSet map[flow.RegisterID]T1,
	largeSet map[flow.RegisterID]T2,
) (
	bool,
	flow.RegisterID,
) {
	for id := range smallSet {
		_, ok := largeSet[id]
		if ok {
			return true, id
		}
	}

	return false, flow.RegisterID{}
}

func intersect[
	T1 any,
	T2 any,
](
	set1 map[flow.RegisterID]T1,
	set2 map[flow.RegisterID]T2,
) (
	bool,
	flow.RegisterID,
) {
	if len(set1) > len(set2) {
		return intersectHelper(set2, set1)
	}

	return intersectHelper(set1, set2)
}
