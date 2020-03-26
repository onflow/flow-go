package cmd

import (
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/model/flow"
)

func constructGenesisQC(block *flow.Block, nodeInfosPub []NodeInfoPub, nodeInfosPriv []NodeInfoPriv, dkgDataPriv DKGDataPriv) {
	signerData := generateQCSignerData(nodeInfosPub, nodeInfosPriv, dkgDataPriv)

	qc, err := run.GenerateGenesisQC(signerData, block)
	if err != nil {
		log.Fatal().Err(err).Msg("generating genesis QC failed")
	}

	writeJSON(filenameGenesisQC, qc)
}

func generateQCSignerData(nsPub []NodeInfoPub, nsPriv []NodeInfoPriv, dkg DKGDataPriv) run.SignerData {
	// nsPub can include external validators, so it can be longer than nsPriv
	if len(nsPub) < len(nsPriv) {
		log.Fatal().Int("len(nsPub)", len(nsPub)).Int("len(nsPriv)", len(nsPriv)).
			Msg("need at least as many staking public keys as staking private keys")
	}

	// length of DKG participants needs to match nsPub, since we run DKG for external and internal validators
	if len(nsPub) != len(dkg.Participants) {
		log.Fatal().Int("len(nsPub)", len(nsPub)).Int("len(dkg.Participants)", len(dkg.Participants)).
			Msg("need exactly the same number of staking public keys as DKG private participants")
	}

	sd := run.SignerData{}

	// the QC will be signed by everyone in nsPriv
	for _, nPriv := range nsPriv {
		// find the corresponding entry in nsPub
		nPub := findNodeInfoPub(nsPub, nPriv.NodeID)
		// find the corresponding entry in dkg
		part := findDKGParticipantPriv(dkg, nPriv.NodeID)

		if nPub.NodeID == flow.ZeroID {
			log.Fatal().Str("Address", nPub.Address).Msg("NodeID must not be zero")
		}

		if nPub.Stake == 0 {
			log.Fatal().Str("NodeID", nPub.NodeID.String()).Msg("Stake must not be 0")
		}

		sd.Signers = append(sd.Signers, run.Signer{
			Identity: flow.Identity{
				NodeID:             nPub.NodeID,
				Address:            nPub.Address,
				Role:               nPub.Role,
				Stake:              nPub.Stake,
				StakingPubKey:      nPub.StakingPubKey,
				RandomBeaconPubKey: part.RandomBeaconPrivKey.PublicKey(),
				NetworkPubKey:      nPub.NetworkPubKey,
			},
			StakingPrivKey:      nPriv.StakingPrivKey,
			RandomBeaconPrivKey: part.RandomBeaconPrivKey,
		})
	}

	sd.DkgPubData = generateDKGPublicData(dkg)

	return sd
}

func findNodeInfoPub(nsPub []NodeInfoPub, nodeID flow.Identifier) NodeInfoPub {
	for _, nPub := range nsPub {
		if nPub.NodeID == nodeID {
			return nPub
		}
	}
	log.Fatal().Str("nodeID", nodeID.String()).Msg("could not find nodeID in public node info")
	return NodeInfoPub{}
}

func findDKGParticipantPriv(dkg DKGDataPriv, nodeID flow.Identifier) DKGParticipantPriv {
	for _, part := range dkg.Participants {
		if part.NodeID == nodeID {
			return part
		}
	}
	log.Fatal().Str("nodeID", nodeID.String()).Msg("could not find nodeID in private DKG data")
	return DKGParticipantPriv{}
}

func generateDKGPublicData(dkg DKGDataPriv) *hotstuff.DKGPublicData {
	dat := hotstuff.DKGPublicData{
		GroupPubKey:           dkg.PubGroupKey,
		IdToDKGParticipantMap: make(map[flow.Identifier]*hotstuff.DKGParticipant, len(dkg.Participants)),
	}

	for _, part := range dkg.Participants {
		dat.IdToDKGParticipantMap[part.NodeID] = &hotstuff.DKGParticipant{
			Id:             part.NodeID,
			PublicKeyShare: part.RandomBeaconPrivKey.PublicKey(),
			DKGIndex:       part.GroupIndex,
		}
	}

	return &dat
}
