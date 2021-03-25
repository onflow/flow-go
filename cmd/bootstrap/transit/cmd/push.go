package cmd

import (
	"context"
	"fmt"
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
	pushCmd.Flags().StringVar(&flagToken, "token", "", "token provided by the Flow team to access the Transit server")
	pushCmd.Flags().StringVar(&flagNodeRole, "role", "", `node role (can be "collection", "consensus", "execution", "verification" or "access")`)

	_ = pushCmd.MarkFlagRequired("token")
	_ = pushCmd.MarkFlagRequired("role")
}

// push uploads public keys to the transit server
func push(cmd *cobra.Command, args []string) {
	log.Info().Msg("running push")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	role, err := flow.ParseRole(flagNodeRole)
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse Flow role")
	}

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

	if role == flow.RoleConsensus {
		err := generateKeys(flagBootDir, nodeID)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to push")
		}
	}

	files := getFilesToUpload(role)
	for _, file := range files {
		fileName := fmt.Sprintf(file, nodeID)
		destination := filepath.Join(flagToken, fileName)
		source := filepath.Join(flagBootDir, fileName)
		err := bucket.UploadFile(ctx, client, destination, source)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to push")
		}
	}
}
