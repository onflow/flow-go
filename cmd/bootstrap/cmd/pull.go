package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/bootstrap/gcs"
)

var (
	flagNetwork    string
	flagBucketName string
)

// pullCmd represents a command to pull parnter node details from the google
// buckets for a specific --network.
var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull partner node details for a specific network",
	Long:  `Pull partner node details for a specific network from the FLOW Google bucket.`,
	Run:   pull,
}

func init() {
	rootCmd.AddCommand(pullCmd)
	addPullCmdFlags()
}

func addPullCmdFlags() {
	pullCmd.Flags().StringVar(&flagNetwork, "network", "", "network name to pull partner node information")
	cmd.MarkFlagRequired(pullCmd, "network")

	pullCmd.Flags().StringVar(&flagBucketName, "bucket", "flow-genesis-bootstrap", "google bucket name")
}

// pull partner node info from google bucket
func pull(cmd *cobra.Command, args []string) {

	log.Info().Msgf("attempting to download partner info for network `%s` from bucket `%s`", flagNetwork, flagBucketName)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	bucket := gcs.NewGoogleBucket(flagBucketName)

	client, err := bucket.NewClient(ctx)
	if err != nil {
		log.Error().Msgf("error trying get new google bucket client: %v", err)
	}
	defer client.Close()

	prefix := fmt.Sprintf("%s-", flagNetwork)
	files, err := bucket.GetFiles(ctx, client, prefix, "")
	if err != nil {
		log.Error().Msgf("error trying list google bucket files: %v", err)
	}
	log.Info().Msgf("found %d files in google bucket", len(files))

	for _, file := range files {
		if strings.Contains(file, "node-info.pub") {
			fullOutpath := filepath.Join(flagOutdir, file)
			log.Printf("downloading %s", file)

			err = bucket.DownloadFile(ctx, client, fullOutpath, file)
			if err != nil {
				log.Error().Msgf("error trying download google bucket file: %v", err)
			}
		}
	}
}
