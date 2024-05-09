package protocol

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// DefaultEpochCommitSafetyThreshold returns the default epoch commit safety
// threshold for each chain ID. Greater threshold values are generally safer,
// but require longer epochs and longer EpochCommit phases. See Params for
// more details on this value.
func DefaultEpochCommitSafetyThreshold(chain flow.ChainID) (uint64, error) {
	switch chain {
	case flow.Mainnet, flow.Testnet, flow.Sandboxnet, flow.Previewnet:
		return 1_000, nil
	case flow.Localnet, flow.Benchnet, flow.BftTestnet, flow.Emulator:
		return 100, nil
	}
	return 0, fmt.Errorf("unkown chain id %s", chain.String())
}
