package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	bootstrap "github.com/onflow/flow-go/cmd/bootstrap/cmd"
	"github.com/onflow/flow-go/ledger/complete/wal"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

var (
	flagToken      string
	flagNodeRole   string
	flagBucketName string
)

// pullCmd represents a command to pull keys and metadata from the Google bucket
var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Fetch public keys and metadata from the transit server",
	Long:  `Fetch keys and metadata from the transit server`,
	Run:   pull,
}

func init() {
	rootCmd.AddCommand(pullCmd)
	addPullCmdFlags()
}

func addPullCmdFlags() {
	pullCmd.Flags().StringVar(&flagToken, "token", "", "token provided by the Flow team to access the Transit server")
	pullCmd.Flags().StringVar(&flagNodeRole, "role", "", `node role (can be "collection", "consensus", "execution", "verification" or "access")`)
	pullCmd.Flags().StringVar(&flagBucketName, "bucket", "", "the name of the Google Bucket provided by the Flow team")

	_ = pullCmd.MarkFlagRequired("token")
	_ = pullCmd.MarkFlagRequired("role")
	_ = pullCmd.MarkFlagRequired("bucket")
}

// pull keys and metadata from the Google bucket
func pull(cmd *cobra.Command, args []string) {
	log.Info().Msg("running pull")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	nodeID, err := readNodeID()
	if err != nil {
		log.Fatal().Err(err).Msg("could not read node ID")
	}

	role, err := flow.ParseRole(flagNodeRole)
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse Flow role")
	}

	// create new bucket instance with Flow Bucket name
	bucket := bootstrap.NewGoogleBucket(flagBucketName)

	// initialize a new client to GCS
	client, err := bucket.NewClient(ctx)
	if err != nil {
		log.Fatal().Msgf("error trying get new google bucket client: %v", err)
	}
	defer client.Close()

	// get files to download from bucket
	prefix := fmt.Sprintf("%s/%s/", flagToken, folderToDownload)
	files, err := bucket.GetFiles(ctx, client, prefix, "")
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get list of files from GCS")
	}
	log.Info().Msgf("found %d files in Google Bucket", len(files))

	// download found files
	for _, file := range files {
		fullOutpath := filepath.Join(flagBootDir, file)

		log.Info().Msgf("downloading: %s", file)
		err = bucket.DownloadFile(ctx, client, fullOutpath, file)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not download google bucket file")
		}
	}

	// download any extra files specific to node role
	extraFiles := getAdditionalFilesToDownload(role, nodeID)
	for _, file := range extraFiles {
		objectName := filepath.Join(flagToken, file)
		fullOutpath := filepath.Join(flagBootDir, filepath.Base(objectName))

		log.Info().Msgf("downloading extra file: %s", objectName)
		err = bucket.DownloadFile(ctx, client, fullOutpath, objectName)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not download google bucket file")
		}
	}

	// move root checkpoint file if node role is execution
	if role == flow.RoleExecution {
		// root.checkpoint is downloaded to <bootstrap folder>/public-root-information after a pull
		rootCheckpointSrc := filepath.Join(flagBootDir, model.DirnamePublicBootstrap, wal.RootCheckpointFilename)
		rootCheckpointDst := filepath.Join(flagBootDir, model.PathRootCheckpoint)

		log.Info().Str("src", rootCheckpointSrc).Str("destination", rootCheckpointDst).Msgf("moving file")
		err := moveFile(rootCheckpointSrc, rootCheckpointDst)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to move root.checkpoint")
		}
	}

	// unwrap consensus node role files
	if role == flow.RoleConsensus {
		err = unWrapFile(nodeID)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to pull")
		}
	}

	// calculate MD5 of rootsnapshot
	rootFile := filepath.Join(flagBootDir, model.PathRootProtocolStateSnapshot)
	rootMD5, err := getFileMD5(rootFile)
	if err != nil {
		log.Fatal().Err(err).Str("file", rootFile).Msg("failed to calculate md5")
	}
	log.Info().Str("md5", rootMD5).Msg("calculated MD5 of protocol snapshot")
}
