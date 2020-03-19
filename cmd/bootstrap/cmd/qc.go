package cmd

import (
	"github.com/dapperlabs/flow-go-sdk/utils/unittest"
	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var genesisBlockFile string
var nodeInfosPubFile string
var nodeInfosPrivFile string
var dkgDataPrivFile string

// qcCmd represents the qc command
var qcCmd = &cobra.Command{
	Use:   "qc",
	Short: "Construct genesis QC (quorum certificate)",
	Run: func(cmd *cobra.Command, args []string) {
		// var block flow.Block
		// readYaml(genesisBlockFile, &block)
		block := unittest.BlockFixture() // TODO replace once block decoding works
		var nodeInfosPub []NodeInfoPub
		readYaml(nodeInfosPubFile, &nodeInfosPub)
		var nodeInfosPriv []NodeInfoPriv
		readYaml(nodeInfosPrivFile, &nodeInfosPriv)
		var dkgDataPriv DKGDataPriv
		readYaml(dkgDataPrivFile, &dkgDataPriv)

		signerData := generateQCSignerData(nodeInfosPub, nodeInfosPriv, dkgDataPriv)

		qc, err := run.GenerateGenesisQC(signerData, block)
		if err != nil {
			log.Fatal().Err(err).Msg("generating genesis QC failed")
		}

		writeYaml("genesis-qc.yml", qc)
	},
}

func init() {
	rootCmd.AddCommand(qcCmd)

	qcCmd.Flags().StringVarP(&genesisBlockFile, "genesis-block", "g", "",
		"Path to a yml file containing the genesis block [required]")
	qcCmd.MarkFlagRequired("genesis-block")

	qcCmd.Flags().StringVarP(&nodeInfosPubFile, "node-infos-pub", "n", "",
		"Path to a yml file containing public staking information for all genesis nodes [required]")
	qcCmd.MarkFlagRequired("node-infos-pub")

	qcCmd.Flags().StringVarP(&nodeInfosPrivFile, "node-infos-priv", "o", "",
		"Path to a yml file containing private staking information for all genesis nodes [required]")
	qcCmd.MarkFlagRequired("node-infos-priv")

	qcCmd.Flags().StringVarP(&dkgDataPrivFile, "dkg-data-priv", "e", "",
		"Path to a yml file containing private dkg data for all genesis nodes [required]")
	qcCmd.MarkFlagRequired("dkg-data-priv")
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
			StakingPrivKey:       nPriv.StakingPrivKey,
			RandomBeaconPrivKeys: part.RandomBeaconPrivKey,
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
		// dkgPart :=
		dat.IdToDKGParticipantMap[part.NodeID] = &hotstuff.DKGParticipant{
			Id:             part.NodeID,
			PublicKeyShare: part.RandomBeaconPrivKey.PublicKey(),
			DKGIndex:       part.GroupIndex,
		}
	}

	return &dat
}
