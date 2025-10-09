package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd"
	cmd2 "github.com/onflow/flow-go/cmd/bootstrap/cmd"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/utils/io"
)

var generateClusterVoteCmd = &cobra.Command{
	Use:   "generate-cluster-block-vote",
	Short: "Generate cluster block vote",
	Run:   generateClusterVote,
}

func init() {
	rootCmd.AddCommand(generateClusterVoteCmd)
	addGenerateClusterVoteCmdFlags()
}

func addGenerateClusterVoteCmdFlags() {
	generateClusterVoteCmd.Flags().StringVarP(&flagOutputDir, "outputDir", "o", "", "ouput directory for vote files; if not set defaults to bootstrap directory")
}

func generateClusterVote(c *cobra.Command, args []string) {
	log.Info().Msg("generating root block vote")

	nodeIDString, err := readNodeID()
	if err != nil {
		log.Fatal().Err(err).Msg("could not read node ID")
	}

	nodeID, err := flow.HexStringToIdentifier(nodeIDString)
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse node ID")
	}

	nodeInfo, err := cmd.LoadPrivateNodeInfo(flagBootDir, nodeID)
	if err != nil {
		log.Fatal().Err(err).Msg("could not load private node info")
	}

	stakingPrivKey := nodeInfo.StakingPrivKey.PrivateKey
	identity := flow.IdentitySkeleton{
		NodeID:        nodeID,
		Address:       nodeInfo.Address,
		Role:          nodeInfo.Role,
		InitialWeight: flow.DefaultInitialWeight,
		StakingPubKey: stakingPrivKey.PublicKey(),
		NetworkPubKey: nodeInfo.NetworkPrivKey.PrivateKey.PublicKey(),
	}

	me, err := local.New(identity, nodeInfo.StakingPrivKey.PrivateKey)
	if err != nil {
		log.Fatal().Err(err).Msg("creating local signer abstraction failed")
	}

	path := filepath.Join(flagBootDir, bootstrap.PathClusteringData)
	// If output directory is specified, use it for the root-clustering.json
	if flagOutputDir != "" {
		path = filepath.Join(flagOutputDir, "root-clustering.json")
	}

	data, err := io.ReadFile(path)
	if err != nil {
		log.Fatal().Err(err).Msg("could not read clustering file")
	}

	var clustering cmd2.IntermediaryClusteringData
	err = json.Unmarshal(data, &clustering)
	if err != nil {
		log.Fatal().Err(err).Msg("could not unmarshal clustering data")
	}

	var myCluster flow.IdentifierList
	for _, assignment := range clustering.Assignments {
		if assignment.Contains(me.NodeID()) {
			myCluster = assignment
		}
	}
	if myCluster == nil {
		log.Fatal().Msg("node not a member of any clusters")
	}
	clusterBlock, err := cluster.CanonicalRootBlock(clustering.EpochCounter, myCluster)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create canonical root cluster block")
	}

	// generate root block vote
	vote, err := verification.NewStakingSigner(me).CreateVote(model.GenesisBlockFromFlow(clusterBlock.ToHeader()))
	if err != nil {
		log.Fatal().Err(err).Msgf("could not create cluster vote for participant %v", me.NodeID())
	}

	voteFile := fmt.Sprintf(bootstrap.PathNodeRootClusterBlockVote, nodeID)

	// By default, use the bootstrap directory for storing the vote file
	voteFilePath := filepath.Join(flagBootDir, voteFile)

	// If output directory is specified, use it for the vote file path
	if flagOutputDir != "" {
		voteFilePath = filepath.Join(flagOutputDir, "root-cluster-block-vote.json")
	}

	if err = io.WriteJSON(voteFilePath, vote); err != nil {
		log.Fatal().Err(err).Msg("could not write vote to file")
	}

	log.Info().Msgf("node %v successfully generated vote file for root cluster block %v", nodeID, clusterBlock.ID())
}
