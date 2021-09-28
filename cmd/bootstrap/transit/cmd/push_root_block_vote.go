package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/bootstrap/gcs"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/io"
)

var pushVoteCmd = &cobra.Command{
	Use:   "push-root-block-vote",
	Short: "Generate and push root block vote",
	Run:   pushVote,
}

func init() {
	rootCmd.AddCommand(pushVoteCmd)
	addPushVoteCmdFlags()
}

func addPushVoteCmdFlags() {
	pullCmd.Flags().StringVarP(&flagToken, "token", "t", "", "token provided by the Flow team to access the Transit server")
	_ = pullCmd.MarkFlagRequired("token")
}

func pushVote(c *cobra.Command, args []string) {
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

	// load DKG private key
	path := fmt.Sprintf(bootstrap.PathRandomBeaconPriv, nodeID)
	data, err := io.ReadFile(filepath.Join(flagBootDir, path))
	if err != nil {
		log.Fatal().Err(err).Msg("could not read DKG private key file")
	}

	var priv dkg.DKGParticipantPriv
	err = json.Unmarshal(data, &priv)
	if err != nil {
		log.Fatal().Err(err).Msg("could not unmarshal DKG private key data")
	}

	randomBeaconPrivKey := priv.RandomBeaconPrivKey.PrivateKey
	stakingPrivKey := nodeInfo.StakingPrivKey.PrivateKey
	identity := &flow.Identity{
		NodeID:        nodeID,
		Address:       nodeInfo.Address,
		Role:          nodeInfo.Role,
		Stake:         0, // TODO: is this okay?
		StakingPubKey: stakingPrivKey.PublicKey(),
		NetworkPubKey: nodeInfo.NetworkPrivKey.PrivateKey.PublicKey(),
	}

	local, err := local.New(identity, nodeInfo.StakingPrivKey.PrivateKey)

	merger := signature.NewCombiner(encodable.ConsensusVoteSigLen, encodable.RandomBeaconSigLen)
	stakingSigner := signature.NewAggregationProvider(encoding.ConsensusVoteTag, local)
	beaconVerifier := signature.NewThresholdVerifier(encoding.RandomBeaconTag)
	beaconSigner := signature.NewThresholdProvider(encoding.RandomBeaconTag, randomBeaconPrivKey)
	beaconStore := signature.NewSingleSignerStore(beaconSigner)
	signer := verification.NewCombinedSigner(nil, stakingSigner, beaconVerifier, merger, beaconStore, nodeID)

	path = filepath.Join(flagBootDir, bootstrap.DirnamePublicBootstrap, bootstrap.FilenameRootBlock)
	data, err = io.ReadFile(path)
	if err != nil {
		log.Fatal().Err(err).Msg("could not read root block file")
	}

	var rootBlock flow.Block
	err = json.Unmarshal(data, &rootBlock)
	if err != nil {
		log.Fatal().Err(err).Msg("could not unmarshal root block data")
	}

	vote, err := signer.CreateVote(model.GenesisBlockFromFlow(rootBlock.Header))
	if err != nil {
		log.Fatal().Err(err).Msg("could not load private node info")
	}

	voteFile := fmt.Sprintf(FilenameRootBlockVote, nodeID)

	if err = io.WriteJSON(filepath.Join(flagBootDir, voteFile), vote); err != nil {
		log.Fatal().Err(err).Msg("could not write vote to file")
	}

	destination := filepath.Join(flagToken, voteFile)
	source := filepath.Join(flagBootDir, voteFile)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// create new bucket instance with Flow Bucket name
	bucket := gcs.NewGoogleBucket(flagBucketName)

	// initialize a new client to GCS
	client, err := bucket.NewClient(ctx)
	if err != nil {
		log.Fatal().Err(err).Msgf("error trying get new google bucket client")
	}
	defer client.Close()

	err = bucket.UploadFile(ctx, client, destination, source)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to upload vote file")
	}

	log.Info().Msg("successfully pushed vote file")
}
