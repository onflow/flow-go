package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/bootstrap/gcs"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

var pushClusterVoteCmd = &cobra.Command{
	Use:   "push-cluster-block-vote",
	Short: "Push cluster block vote",
	Run:   pushClusterVote,
}

func init() {
	rootCmd.AddCommand(pushClusterVoteCmd)
	addPushClusterVoteCmdFlags()
}

func addPushClusterVoteCmdFlags() {
	defaultVoteFilePath := fmt.Sprintf(bootstrap.PathNodeRootClusterBlockVote, "<node_id>")
	pushClusterVoteCmd.Flags().StringVarP(&flagToken, "token", "t", "", "token provided by the Flow team to access the Transit server")
	pushClusterVoteCmd.Flags().StringVarP(&flagVoteFile, "vote-file", "v", "", fmt.Sprintf("path under bootstrap directory of the vote file to upload (default: %s)", defaultVoteFilePath))
	pushClusterVoteCmd.Flags().StringVarP(&flagVoteFilePath, "vote-file-dir", "d", "", "directory for vote file to upload, ONLY for vote files outside the bootstrap directory")
	pushClusterVoteCmd.Flags().StringVarP(&flagBucketName, "bucket-name", "g", "flow-genesis-bootstrap", `bucket for pushing root cluster block vote files`)

	_ = pushClusterVoteCmd.MarkFlagRequired("token")
	pushClusterVoteCmd.MarkFlagsMutuallyExclusive("vote-file", "vote-file-dir")
}

func pushClusterVote(c *cobra.Command, args []string) {
	nodeIDString, err := readNodeID()
	if err != nil {
		log.Fatal().Err(err).Msg("could not read node ID")
	}

	nodeID, err := flow.HexStringToIdentifier(nodeIDString)
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse node ID")
	}

	voteFile := flagVoteFile

	// If --vote-file-dir is not specified, use the bootstrap directory
	voteFilePath := filepath.Join(flagBootDir, voteFile)

	// if --vote-file is not specified, use default file name within bootstrap directory
	if voteFile == "" {
		voteFile = fmt.Sprintf(bootstrap.PathNodeRootClusterBlockVote, nodeID)
		voteFilePath = filepath.Join(flagBootDir, voteFile)
	}

	// If vote-file-dir is specified, use it to construct the full path to the vote file (with default file name)
	if flagVoteFilePath != "" {
		voteFilePath = filepath.Join(flagVoteFilePath, "root-cluster-block-vote.json")
	}

	destination := filepath.Join(flagToken, fmt.Sprintf(bootstrap.FilenameRootClusterBlockVote, nodeID))

	log.Info().Msg("pushing root cluster block vote")

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

	err = bucket.UploadFile(ctx, client, destination, voteFilePath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to upload cluster vote file")
	}

	log.Info().Msg("successfully pushed cluster vote file")
}
