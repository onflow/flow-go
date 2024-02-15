package common

import (
	"fmt"

	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

const (
	getInfoForProposedNodesScript = `
		import FlowIDTableStaking from 0xIDENTITYTABLEADDRESS
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
	contracts := systemcontracts.SystemContractsForChain(flow.ChainID(fmt.Sprintf("flow-%s", network)))

	return []byte(
		templates.ReplaceAddresses(
			getInfoForProposedNodesScript,
			contracts.AsTemplateEnv(),
		),
	), nil
}
