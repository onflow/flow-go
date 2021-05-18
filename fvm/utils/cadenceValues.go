package utils

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"
)

func OptionalCadenceValueToAddressSlice(value cadence.Value) (addresses []common.Address, ok bool) {
	optV, ok := value.(cadence.Optional)
	if !ok {
		return nil, false
	}
	v, ok := optV.Value.(cadence.Array)
	if !ok {
		return nil, false
	}

	for _, i := range v.Values {
		a, ok := i.(cadence.Address)
		if !ok {
			return nil, false
		}
		addresses = append(addresses, common.Address(a))
	}
	return addresses, true
}
