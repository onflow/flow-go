package cmd

import (
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/model/flow"
)

func constructGenesisBlock(stateComm flow.StateCommitment, nodes []NodeInfoPub, dkg DKGDataPub) flow.Block {
	seal := run.GenerateRootSeal(stateComm)
	identityList := generateIdentityList(nodes, dkg)
	block := run.GenerateRootBlock(identityList, seal)

	writeJSON(FilenameGenesisBlock, block)

	return block
}

func generateIdentityList(infos []NodeInfoPub, dkgDataPub DKGDataPub) flow.IdentityList {
	var list flow.IdentityList
	list = make([]*flow.Identity, 0, len(infos))

	for _, info := range infos {
		ident := flow.Identity{
			NodeID:        info.NodeID,
			Address:       info.Address,
			Role:          info.Role,
			Stake:         info.Stake,
			StakingPubKey: info.StakingPubKey,
			NetworkPubKey: info.NetworkPubKey,
		}

		if info.Role == flow.RoleConsensus {
			ident.RandomBeaconPubKey = findDKGParticipantPub(dkgDataPub, info.NodeID).RandomBeaconPubKey
		}

		list = append(list, &ident)
	}

	return list
}

func findDKGParticipantPub(dkg DKGDataPub, nodeID flow.Identifier) DKGParticipantPub {
	for _, part := range dkg.Participants {
		if part.NodeID == nodeID {
			return part
		}
	}
	log.Fatal().Str("nodeID", nodeID.String()).Msg("could not find nodeID in public DKG data")
	return DKGParticipantPub{}
}
