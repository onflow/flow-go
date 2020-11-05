package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	flagNetwork    string
	flagBucketName string
)

// pullCmd represents a command to pull parnter node details from the google
// buckets for a specific --network.
var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull parnter node details for a specific network",
	Long:  `Pull parnter node details for a specific network from the FLOW Google bucket.`,
	Run:   pull,
}

func init() {
	rootCmd.AddCommand(pullCmd)
	addPullCmdFlags()
}

func addPullCmdFlags() {
	pullCmd.Flags().StringVar(&flagNetwork, "network", "", "network name to pull partner node information")
	_ = pullCmd.MarkFlagRequired("network")

	pullCmd.Flags().StringVar(&flagBucketName, "bucket", "flow-genesis-bootstrap", "google bucket name")
	_ = pullCmd.MarkFlagRequired("bucket")
}

// pull partner node info from google bucket
func pull(cmd *cobra.Command, args []string) {
	log.Info().Msgf("Attempting to download partner info for network `%s` from bucket `%s`", flagNetwork, flagBucketName)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	bucket := NewGoogleBucket(flagBucketName)

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

	for _, file := range files {
		fullOutpath := filepath.Join(flagOutdir, file)
		log.Info().Msgf("Downloading %s", file)

		err = bucket.DownloadFile(ctx, client, fullOutpath, file)
		if err != nil {
			log.Error().Msgf("error trying download google bucket file: %v", err)
		}
	}
}
