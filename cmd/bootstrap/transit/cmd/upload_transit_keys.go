package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/bootstrap/gcs"
	"github.com/onflow/flow-go/model/flow"
)

// pushTransitKeyCmd represents a command to upload public keys to the transit server
var pushTransitKeyCmd = &cobra.Command{
	Use:   "push-transit-key",
	Short: "Upload transit key for a consensus node",
	Long:  `Upload transit key for a consensus node`,
	Run:   pushTransitKey,
}

func init() {
	rootCmd.AddCommand(pushTransitKeyCmd)
	addUploadTransitKeysCmdFlags()
}

func addUploadTransitKeysCmdFlags() {
	pushTransitKeyCmd.Flags().StringVarP(&flagToken, "token", "t", "", "token provided by the Flow team")
	err := pushTransitKeyCmd.MarkFlagRequired("token")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize")
	}
}

// pushTransitKey uploads transit keys to the Flow server
func pushTransitKey(_ *cobra.Command, _ []string) {

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

	if nodeInfo.Role != flow.RoleConsensus {
		log.Info().Str("role", nodeInfo.Role.String()).Msgf("only consensus nodes are required to push transit keys, exiting.")
		os.Exit(0)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	log.Info().Msg("generating transit keys")

	err = generateKeys(flagBootDir, nodeIDString)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to push")
	}

	// create new bucket instance with Flow Bucket name
	bucket := gcs.NewGoogleBucket(flagBucketName)

	// initialize a new client to GCS
	client, err := bucket.NewClient(ctx)
	if err != nil {
		log.Fatal().Err(err).Msgf("error trying get new google bucket client")
	}
	defer client.Close()

	log.Info().Msg("attempting to push transit public key to the transit servers")
	fileName := fmt.Sprintf(FilenameTransitKeyPub, nodeID)
	destination := filepath.Join(flagToken, fileName)
	source := filepath.Join(flagBootDir, fileName)
	err = bucket.UploadFile(ctx, client, destination, source)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to push")
	}

	log.Info().Msg("successfully pushed transit public key to the transit servers")
}
