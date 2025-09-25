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
	pullRootBlockCmd.Flags().StringVarP(&flagBucketName, "bucket-name", "g", "flow-genesis-bootstrap", `bucket for pulling root block`)
	pullRootBlockCmd.Flags().StringVarP(&flagOutputDir, "outputDir", "o", "", "output directory for root block file; if not set defaults to bootstrap directory")
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

	rootBlockFile := filepath.Join(flagToken, bootstrap.PathRootBlockData)
	fullRootBlockPath := filepath.Join(flagBootDir, bootstrap.PathRootBlockData)
	if flagOutputDir != "" {
		fullRootBlockPath = filepath.Join(flagOutputDir, "root-block.json")
	}

	log.Info().Str("source", rootBlockFile).Str("dest", fullRootBlockPath).Msgf("downloading root block file from transit servers")
	err = bucket.DownloadFile(ctx, client, fullRootBlockPath, rootBlockFile)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not download google bucket file")
	}

	log.Info().Msg("successfully downloaded root block ")

	objectName := filepath.Join(flagToken, fmt.Sprintf(FilenameRandomBeaconCipher, nodeID))

	// By default, use the bootstrap directory for the random beacon download & unwrapping
	fullRandomBeaconPath := filepath.Join(flagBootDir, filepath.Base(objectName))
	unWrappedRandomBeaconPath := filepath.Join(
		flagBootDir,
		fmt.Sprintf(bootstrap.PathRandomBeaconPriv, nodeID),
	)

	// If output directory is specified, use it for the random beacon path
	// this will set the path used to download the random beacon file and unwrap it
	if flagOutputDir != "" {
		fullRandomBeaconPath = filepath.Join(flagOutputDir, filepath.Base(objectName))
		unWrappedRandomBeaconPath = filepath.Join(flagOutputDir, bootstrap.FilenameRandomBeaconPriv)
	}

	log.Info().Msgf("downloading random beacon key: %s", objectName)

	err = bucket.DownloadFile(ctx, client, fullRandomBeaconPath, objectName)
	if err != nil {
		log.Fatal().Err(err).Msg("could not download random beacon key file from google bucket")
	} else {
		err = unWrapFile(flagBootDir, nodeID, flagOutputDir, unWrappedRandomBeaconPath)
		if err != nil {
			log.Fatal().Err(err).Msg("could not unwrap random beacon file")
		}
		log.Info().Msg("successfully downloaded and unwrapped random beacon private key")
	}
}
