package protocol

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

type SafetyParams struct {
	FinalizationSafetyThreshold uint64
	EpochExtensionViewCount     uint64
}

// DefaultEpochSafetyParams returns the default epoch safety
// threshold for each chain ID. Greater threshold values are generally safer,
// but require longer epochs and longer EpochCommit phases. See Params for
// more details on this value.
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
			EpochExtensionViewCount:     4_000, // approximately 1 hour
		}, nil
	}
	return SafetyParams{}, fmt.Errorf("unkown chain id %s", chain.String())
}
