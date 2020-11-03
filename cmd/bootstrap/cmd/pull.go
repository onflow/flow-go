package cmd

import (
	"context"
	"time"

	"cloud.google.com/go/storage"
	"github.com/spf13/cobra"
	"google.golang.org/api/iterator"
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
	bucket := "flow-genesis-bootstrap"

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Error().Msgf("storage.NewClient: %v", err)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	it := client.Bucket(bucket).Objects(ctx, &storage.Query{
		Prefix: "candidate-0-",
	})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Error().Msgf("Bucket(%q).Objects: %v", bucket, err)
			return
		}
		log.Info().Msg(attrs.Name)
	}

	log.Info().Msg("Done")
}
