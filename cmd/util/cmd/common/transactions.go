package common

import (
	"fmt"

	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

const (
	getInfoForProposedNodesScript = `
		import FlowIDTableStaking from "FlowIDTableStaking"
		access(all) fun main(): [FlowIDTableStaking.NodeInfo] {
			let nodeIDs = FlowIDTableStaking.getProposedNodeIDs()
		
			var infos: [FlowIDTableStaking.NodeInfo] = []
			for nodeID in nodeIDs {
				let node = FlowIDTableStaking.NodeInfo(nodeID: nodeID)
				infos.append(node)
			}
		
			return infos
	}`

	getInfoForCandidateAccessNodesScript = `
		import FlowIDTableStaking from "FlowIDTableStaking"
		access(all) fun main(): [FlowIDTableStaking.NodeInfo] {
		let candidateNodes = FlowIDTableStaking.getCandidateNodeList()
		let candidateAccessNodes = candidateNodes[UInt8(5)]!

        let nodeInfos: [FlowIDTableStaking.NodeInfo] = []
        for nodeID in candidateAccessNodes.keys {
            let nodeInfo = FlowIDTableStaking.NodeInfo(nodeID: nodeID)
            nodeInfos.append(nodeInfo)
        }

        return nodeInfos
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

// GetNodeInfoForCandidateNodesScript returns a script that will return an array of FlowIDTableStaking.NodeInfo for each
// node in the candidate table (nodes which have staked but not yet chosen by the network).
func GetNodeInfoForCandidateNodesScript(network string) ([]byte, error) {
	contracts := systemcontracts.SystemContractsForChain(flow.ChainID(fmt.Sprintf("flow-%s", network)))

	return []byte(
		templates.ReplaceAddresses(
			getInfoForCandidateAccessNodesScript,
			contracts.AsTemplateEnv(),
		),
	), nil
}
