package cmd

import (
	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
)

func constructGenesisBlock(stateComm flow.StateCommitment, nodes []model.NodeInfo, dkg model.DKGData) flow.Block {
	seal := run.GenerateRootSeal(stateComm)
	identityList := generateIdentityList(nodes, dkg)
	block := run.GenerateRootBlock(identityList, seal)

	writeJSON(model.FilenameGenesisBlock, block)

	return block
}

func generateIdentityList(nodes []model.NodeInfo, dkgData model.DKGData) flow.IdentityList {

	list := make([]*flow.Identity, 0, len(nodes))

	for _, node := range nodes {
		ident := node.Identity()
		if node.Role == flow.RoleConsensus {
			ident.RandomBeaconPubKey = findDKGParticipant(dkgData, node.NodeID).KeyShare.PublicKey()
		}

		list = append(list, ident)
	}

	return list
}
