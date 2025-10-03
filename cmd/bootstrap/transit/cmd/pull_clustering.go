package cmd

import (
	"context"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/bootstrap/gcs"
	"github.com/onflow/flow-go/model/bootstrap"
)

var pullClusteringCmd = &cobra.Command{
	Use:   "pull-clustering",
	Short: "Pull epoch clustering",
	Run:   pullClustering,
}

func init() {
	rootCmd.AddCommand(pullClusteringCmd)
	addPullClusteringFlags()
}

func addPullClusteringFlags() {
	pullClusteringCmd.Flags().StringVarP(&flagToken, "token", "t", "", "token provided by the Flow team to access the Transit server")
	pullClusteringCmd.Flags().StringVarP(&flagBucketName, "bucket-name", "g", "flow-genesis-bootstrap", `bucket for pulling root clustering`)
	pullClusteringCmd.Flags().StringVarP(&flagOutputDir, "outputDir", "o", "", "output directory for clustering file; if not set defaults to bootstrap directory")
	_ = pullClusteringCmd.MarkFlagRequired("token")
}

func pullClustering(c *cobra.Command, args []string) {
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

	log.Info().Msg("downloading clustering assignment")

	clusteringFile := filepath.Join(flagToken, bootstrap.PathClusteringData)
	fullClusteringPath := filepath.Join(flagBootDir, bootstrap.PathClusteringData)
	if flagOutputDir != "" {
		fullClusteringPath = filepath.Join(flagOutputDir, "root-clustering.json")
	}

	log.Info().Str("source", clusteringFile).Str("dest", fullClusteringPath).Msgf("downloading clustering file from transit servers")
	err = bucket.DownloadFile(ctx, client, fullClusteringPath, clusteringFile)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not download google bucket file")
	}

	log.Info().Msg("successfully downloaded clustering")
}
