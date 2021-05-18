package utils

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"
)

func OptionalCadenceValueToAddressSlice(value cadence.Value) (addresses []common.Address, ok bool) {

	// cast to optional
	optV, ok := value.(cadence.Optional)
	if !ok {
		return nil, false
	}

	// cast to array
	v, ok := optV.Value.(cadence.Array)
	if !ok {
		return nil, false
	}

	// parse addresses
	for _, i := range v.Values {
		a, ok := i.(cadence.Address)
		if !ok {
			return nil, false
		}
		addresses = append(addresses, common.Address(a))
	}
	return addresses, true
}
