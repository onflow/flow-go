package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"
)

var (
	flagNetwork string
)

// pullCmd represents a command to pull parnter node details from the google
// buckets for a specific --network.
var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull parnter node details for a specific network",
	Long:  `Pull parnter node details for a specific network from the FLOW Google bucket.`,
	Run:   pullPartners,
}

func init() {
	rootCmd.AddCommand(pullCmd)
	addPullCmdFlags()
}

func addPullCmdFlags() {
	// add flags here
}

func pullPartners(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	bucket := NewGoogleBucket("flow-genesis-bootstrap")

	client, err := bucket.NewClient(ctx)
	if err != nil {
		log.Error().Msgf("error trying get new google bucket client: %v", err)
	}
	defer client.Close()

	files, err := bucket.GetFiles(ctx, "mainnet-1-", "")
	if err != nil {
		log.Error().Msgf("error trying list google bucket files: %v", err)
	}

	for _, file := range files {
		log.Info().Msg(file)
	}
}
