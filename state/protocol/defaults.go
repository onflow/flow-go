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
			EpochExtensionViewCount:     600, // approximately 10 minutes
		}, nil
	}
	return SafetyParams{}, fmt.Errorf("unkown chain id %s", chain.String())
}

// RandomBeaconSafetyThreshold defines a production network safety threshold for random beacon protocol based on the size
// of the DKG committee 𝒟 which is a subset of consensus committee 𝒞.
// An honest supermajority of consensus nodes must contain enough successful DKG participants
// (about |𝒟|/2) to produce a valid group signature for the random beacon [1, 3]. Therefore, we have the approximate
// lower bound |𝒟|/2. This is a lower bound, unsuited for decentralized production networks.
// To reject configurations that are vulnerable to liveness failures, the protocol uses the threshold `t_safety`
// (heuristic, see [2]), which is implemented on the smart contract level. In a nutshell, the cardinality of intersection 𝒟 ∩ 𝒞
// (wrt both sets 𝒟 ∩ 𝒞) should be well above 70%, values in the range 70-62% should be considered for short-term
// recovery cases. Values of 62% or lower are not recommended for any
// production network, as single-node crashes are already enough to halt consensus.
// For further details, see
//   - godoc for [flow.DKGIndexMap]
//   - [1] https://www.notion.so/flowfoundation/Threshold-Signatures-7e26c6dd46ae40f7a83689ba75a785e3?pvs=4
//   - [2] https://www.notion.so/flowfoundation/DKG-contract-success-threshold-86c6bf2b92034855b3c185d7616eb6f1?pvs=4
//   - [3] https://www.notion.so/flowfoundation/Architecture-for-Concurrent-Vote-Processing-41704666bc414a03869b70ba1043605f?pvs=4
func RandomBeaconSafetyThreshold(dkgCommitteeSize uint) uint {
	return uint(0.62 * float64(dkgCommitteeSize))
}
