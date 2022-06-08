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

var pushVoteCmd = &cobra.Command{
	Use:   "push-root-block-vote",
	Short: "Push root block vote",
	Run:   pushVote,
}

func init() {
	rootCmd.AddCommand(pushVoteCmd)
	addPushVoteCmdFlags()
}

func addPushVoteCmdFlags() {
	defaultVoteFilePath := fmt.Sprintf(bootstrap.PathNodeRootBlockVote, "<node_id>")
	pushVoteCmd.Flags().StringVarP(&flagToken, "token", "t", "", "token provided by the Flow team to access the Transit server")
	pushVoteCmd.Flags().StringVarP(&flagVoteFile, "vote-file", "v", "", fmt.Sprintf("path under bootstrap directory of the vote file to upload (default: %s)", defaultVoteFilePath))

	_ = pushVoteCmd.MarkFlagRequired("token")
}

func pushVote(c *cobra.Command, args []string) {
	nodeIDString, err := readNodeID()
	if err != nil {
		log.Fatal().Err(err).Msg("could not read node ID")
	}

	nodeID, err := flow.HexStringToIdentifier(nodeIDString)
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse node ID")
	}

	voteFile := flagVoteFile
	if voteFile == "" {
		voteFile = fmt.Sprintf(bootstrap.PathNodeRootBlockVote, nodeID)
	}

	destination := filepath.Join(flagToken, fmt.Sprintf(bootstrap.FilenameRootBlockVote, nodeID))
	source := filepath.Join(flagBootDir, voteFile)

	log.Info().Msg("pushing root block vote")

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
