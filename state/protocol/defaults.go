package protocol

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// SafetyParams contains the safety parameters for the protocol related to the epochs.
// For extra details, refer to documentation of protocol.KVStoreReader.
type SafetyParams struct {
	FinalizationSafetyThreshold uint64
	EpochExtensionViewCount     uint64
}

// DefaultEpochSafetyParams returns the default epoch safety parameters
// for each chain ID.
func DefaultEpochSafetyParams(chain flow.ChainID) (SafetyParams, error) {
	switch chain {
	case flow.Mainnet, flow.Testnet, flow.Sandboxnet, flow.Previewnet:
		return SafetyParams{
			FinalizationSafetyThreshold: 1_000,
			EpochExtensionViewCount:     100_000, // approximately 1 day
		}, nil
	case flow.Localnet, flow.Benchnet, flow.BftTestnet, flow.Emulator:
		return SafetyParams{
			FinalizationSafetyThreshold: 100,
			EpochExtensionViewCount:     690, // approximately 10 minutes
		}, nil
	}
	return SafetyParams{}, fmt.Errorf("unkown chain id %s", chain.String())
}
