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
// of the random beacon committee ℛ and the DKG committee 𝒟.
//
// We recall that the random beacon committee ℛ is defined as the subset of the consensus committee (ℛ ⊆ 𝒞),
// and the DKG committee (ℛ ⊆ 𝒟) which _successfully_ completed the DKG and is able to contribute with a random beacon share.
//
// An honest supermajority of consensus nodes must contain enough successful DKG participants
// (about n/2 + 1) to produce a valid group signature for the random beacon at each block [1, 3].
// Therefore, we have the approximate lower bound |ℛ| >= n/2 + 1 = |𝒟|/2 + 1 = len(DKGIndexMap)/2 + 1.
// Operating close to this lower bound would require that every random beacon key-holder ϱ ∈ ℛ remaining in the consensus committee is honest
// (incl. quickly responsive) *all the time*. This is a lower bound, unsuited for decentralized production networks.
// To reject configurations that are vulnerable to liveness failures, the protocol uses the threshold `t_safety`
// (heuristic, see [2]), which is implemented on the smart contract level.
// In a nutshell, |ℛ| and therefore |𝒟 ∩ 𝒞| (given that |ℛ| <= |𝒟 ∩ 𝒞|) should be well above 70% * |𝒟| = 0.7 * n,
// values in the range 70%-62% of |𝒟|  should be considered for short-term recovery cases.
// Values of 62% * |𝒟| or lower (i.e. |ℛ| ≤ 0.62·|𝒟|) are not recommended for any
// production network, as single-node crashes are already enough to halt consensus.
//
// For further details, see
//   - godoc for [flow.DKGIndexMap]
//   - [1] https://www.notion.so/flowfoundation/Threshold-Signatures-7e26c6dd46ae40f7a83689ba75a785e3?pvs=4
//   - [2] https://www.notion.so/flowfoundation/DKG-contract-success-threshold-86c6bf2b92034855b3c185d7616eb6f1?pvs=4
//   - [3] https://www.notion.so/flowfoundation/Architecture-for-Concurrent-Vote-Processing-41704666bc414a03869b70ba1043605f?pvs=4
func RandomBeaconSafetyThreshold(dkgCommitteeSize uint) uint {
	return uint(0.62 * float64(dkgCommitteeSize))
}
