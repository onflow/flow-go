package common

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

const (
	DefaultAccessNodeIDSMinimumMainnet = 2
	DefaultAccessNodeIDSMinimum        = 1
)

// ValidateAccessNodeIDSFlag validates flag --access-node-ids and converts all ids to flow.Identifier
func ValidateAccessNodeIDSFlag(accessNodeIDS []string, chainID flow.ChainID, snapshot protocol.Snapshot) ([]flow.Identifier, error) {
	if chainID == flow.Mainnet {
		return validateFlagsMainNet(accessNodeIDS)
	}

	return validateFlags(accessNodeIDS, snapshot)
}

// validateFlags will validate access-node-ids flag
// 1. If * (all flag) is used return IDs of all AN's in the identity table
// 2. Ensure minimum number of AN IDs DefaultAccessNodeIDSMinimum provided
func validateFlags(accessNodeIDS []string, snapshot protocol.Snapshot) ([]flow.Identifier, error) {
	if accessNodeIDS[0] == "*" {
		anIDS, err := DefaultAccessNodeIDS(snapshot)
		if err != nil {
			return nil, fmt.Errorf("failed to get default access node ids %w", err)
		}

		return anIDS, nil
	}

	if len(accessNodeIDS) < DefaultAccessNodeIDSMinimum {
		return nil, fmt.Errorf("invalid flag --access-node-ids expected atleast %d IDs got %d", DefaultAccessNodeIDSMinimum, len(accessNodeIDS))
	}

	return convertIDS(accessNodeIDS)
}

// validateFlagsMainNet will validate access-node-ids flag for mainnet
// 1. Disallow the * (all flag), it should never used for mainnet
// 2. Ensure minimum number of AN IDs DefaultAccessNodeIDSMinimumMainnet provided
func validateFlagsMainNet(accessNodeIDS []string) ([]flow.Identifier, error) {
	if accessNodeIDS[0] == "*" {
		return nil, fmt.Errorf("all ('*') can not be used as a value on network %s, please specify access node IDs explicitly", flow.Mainnet)
	}

	if len(accessNodeIDS) < DefaultAccessNodeIDSMinimumMainnet {
		return nil, fmt.Errorf("invalid flag --access-node-ids expected atleast %d IDs got %d for chainID %s", DefaultAccessNodeIDSMinimumMainnet, len(accessNodeIDS), flow.Mainnet)
	}

	return convertIDS(accessNodeIDS)
}

// convertIDS converts a list of access node id hex strings to flow.Identifier
func convertIDS(accessNodeIDS []string) ([]flow.Identifier, error) {
	anIDS, err := FlowIDFromHexString(accessNodeIDS...)
	if err != nil {
		return nil, fmt.Errorf("failed to convert access node ID(s) into flow identifier(s) %w", err)
	}

	return anIDS, nil
}
