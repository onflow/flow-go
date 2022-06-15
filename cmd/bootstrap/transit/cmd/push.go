package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/bootstrap/gcs"
	"github.com/onflow/flow-go/model/flow"
)

// pushCmd represents a command to upload public keys to the transit server
var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Upload public keys to the transit server",
	Long:  `Upload public keys to the transit server`,
	Run:   push,
}

func init() {
	rootCmd.AddCommand(pushCmd)
	addPushCmdFlags()
}

func addPushCmdFlags() {
	pushCmd.Flags().StringVarP(&flagToken, "token", "t", "", "token provided by the Flow team to access the Transit server")
	pushCmd.Flags().StringVarP(&flagNodeRole, "role", "r", "", `node role (can be "collection", "consensus", "execution", "verification" or "access")`)
	_ = pushCmd.MarkFlagRequired("token")
}

// push uploads public keys to the transit server
func push(_ *cobra.Command, _ []string) {
	if flagNodeRole != flow.RoleConsensus.String() {
		log.Info().Str("role", flagNodeRole).Msgf("only consensus nodes are required to push transit keys, exiting.")
		os.Exit(0)
	}

	log.Info().Msg("running push")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	nodeID, err := readNodeID()
	if err != nil {
		log.Fatal().Err(err).Msg("could not read node ID")
	}

	// create new bucket instance with Flow Bucket name
	bucket := gcs.NewGoogleBucket(flagBucketName)

	// initialize a new client to GCS
	client, err := bucket.NewClient(ctx)
	if err != nil {
		log.Fatal().Err(err).Msgf("error trying get new google bucket client")
	}
	defer client.Close()

	err = generateKeys(flagBootDir, nodeID)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to push")
	}

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
