package common

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

const (
	getInfoForProposedNodesScript = `
		import FlowIDTableStaking from 0x%s
		pub fun main(): [FlowIDTableStaking.NodeInfo] {
			let nodeIDs = FlowIDTableStaking.getProposedNodeIDs()
		
			var infos: [FlowIDTableStaking.NodeInfo] = []
			for nodeID in nodeIDs {
				let node = FlowIDTableStaking.NodeInfo(nodeID: nodeID)
				infos.append(node)
			}
		
			return infos
	}`
)

// GetNodeInfoForProposedNodesScript returns a script that will return an array of FlowIDTableStaking.NodeInfo for each
// node in the proposed table.
func GetNodeInfoForProposedNodesScript(network string) ([]byte, error) {
	contracts, err := systemcontracts.SystemContractsForChain(flow.ChainID(fmt.Sprintf("flow-%s", network)))
	if err != nil {
		return nil, fmt.Errorf("failed to get system contracts for network (%s): %w", network, err)
	}

	//NOTE: The FlowIDTableStaking contract is deployed to the same account as the Epoch contract
	return []byte(fmt.Sprintf(getInfoForProposedNodesScript, contracts.Epoch.Address)), nil
}
