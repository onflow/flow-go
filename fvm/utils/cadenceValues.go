package utils

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/model/flow"
)

func FlowAddressSliceToCadenceAddressSlice(addresses []flow.Address) []common.Address {
	adds := make([]common.Address, 0, len(addresses))
	for _, a := range addresses {
		adds = append(adds, common.Address(a))
	}
	return adds
}

func AddressSliceToCadenceValue(addresses []common.Address) cadence.Value {
	adds := make([]cadence.Value, 0, len(addresses))
	for _, a := range addresses {
		adds = append(adds, cadence.NewAddress(a))
	}
	return cadence.NewArray(adds)
}

func CadenceValueToAddressSlice(value cadence.Value) (addresses []common.Address, ok bool) {

	// cast to array
	v, ok := value.(cadence.Array)
	if !ok {
		return nil, false
	}

	// parse addresses
	for _, value := range v.Values {
		a, ok := value.(cadence.Address)
		if !ok {
			return nil, false
		}
		addresses = append(addresses, common.Address(a))
	}
	return addresses, true
}

// CadenceValueToWeights converts a cadence value to a map of weights used for metering
func CadenceValueToWeights(value cadence.Value) (map[uint]uint64, bool) {
	result := make(map[uint]uint64)

	dict, ok := value.(cadence.Dictionary)
	if !ok {
		return nil, false
	}

	for _, p := range dict.Pairs {
		key, ok := p.Key.(cadence.UInt64)
		if !ok {
			return nil, false
		}

		value, ok := p.Value.(cadence.UInt64)
		if !ok {
			return nil, false
		}

		result[uint(key.ToGoValue().(uint64))] = uint64(value)
	}

	return result, true
}
