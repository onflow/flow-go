package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/bootstrap/gcs"
	"github.com/onflow/flow-go/model/bootstrap"
)

var pullRootBlockCmd = &cobra.Command{
	Use:   "pull-root-block",
	Short: "Pull root block and private Random Beacon key",
	Run:   pullRootBlock,
}

func init() {
	rootCmd.AddCommand(pullRootBlockCmd)
	addPullRootBlockCmdFlags()
}

func addPullRootBlockCmdFlags() {
	pullRootBlockCmd.Flags().StringVarP(&flagToken, "token", "t", "", "token provided by the Flow team to access the Transit server")
	_ = pullRootBlockCmd.MarkFlagRequired("token")
}

func pullRootBlock(c *cobra.Command, args []string) {
	nodeID, err := readNodeID()
	if err != nil {
		log.Fatal().Err(err).Msg("could not read node ID")
	}

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

	log.Info().Msg("downloading root block")

	file := filepath.Join(flagToken, bootstrap.PathRootBlockData)
	fullOutpath := filepath.Join(flagBootDir, bootstrap.PathRootBlockData)

	log.Info().Str("source", bootstrap.PathRootBlockData).Str("dest", fullOutpath).Msgf("downloading root block file from transit servers")
	err = bucket.DownloadFile(ctx, client, fullOutpath, file)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not download google bucket file")
	}

	objectName := filepath.Join(flagToken, fmt.Sprintf(FilenameRandomBeaconCipher, nodeID))
	fullOutpath = filepath.Join(flagBootDir, filepath.Base(objectName))

	log.Info().Msgf("downloading random beacon key: %s", objectName)

	err = bucket.DownloadFile(ctx, client, fullOutpath, objectName)
	if err != nil {
		log.Fatal().Err(err).Msg("could not download file from google bucket")
	}

	err = unWrapFile(flagBootDir, nodeID)
	if err != nil {
		log.Fatal().Err(err).Msg("could not unwrap random beacon file")
	}

	log.Info().Msg("successfully downloaded root block and random beacon key")
}
