package common

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// ValidateAccessNodeIDSFlag validates flag --access-node-ids and converts all ids to flow.Identifier
func ValidateAccessNodeIDSFlag(accessNodeIDS []string, chainID flow.ChainID, snapshot protocol.Snapshot) ([]flow.Identifier, error) {
	// disallow all flag for mainnet
	if accessNodeIDS[0] == "*" && chainID == flow.Mainnet {
		return nil, fmt.Errorf("all ('*') can not be used as a value on network %s, please specify access node IDs explicitly.", flow.Mainnet)
	}

	if accessNodeIDS[0] == "*" {
		anIDS, err := DefaultAccessNodeIDS(snapshot)
		if err != nil {
			return nil, fmt.Errorf("failed to get default access node ids %w", err)
		}

		return anIDS, nil
	} else if len(accessNodeIDS) < DefaultAccessNodeIDSMinimum {
		return nil, fmt.Errorf("invalid flag --access-node-ids expected atleast %d IDs got %d", DefaultAccessNodeIDSMinimum, len(accessNodeIDS))
	} else {
		anIDS, err := FlowIDFromHexString(accessNodeIDS...)
		if err != nil {
			return nil, fmt.Errorf("failed to convert access node ID(s) into flow identifier(s) %w", err)
		}

		return anIDS, nil
	}
}
