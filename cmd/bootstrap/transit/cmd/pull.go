package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/bootstrap/gcs"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
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
	pullCmd.Flags().StringVarP(&flagToken, "token", "t", "", "token provided by the Flow team to access the Transit server")
	pullCmd.Flags().StringVarP(&flagNodeRole, "role", "r", "", `node role (can be "collection", "consensus", "execution", "verification" or "access")`)
	pullCmd.Flags().DurationVar(&flagTimeout, "timeout", time.Second*300, `timeout for pull`)

	_ = pullCmd.MarkFlagRequired("token")
	_ = pullCmd.MarkFlagRequired("role")
}

// pull keys and metadata from the Google bucket
func pull(cmd *cobra.Command, args []string) {
	log.Info().Msg("running pull")

	nodeID, err := readNodeID()
	if err != nil {
		log.Fatal().Err(err).Msg("could not read node ID")
	}

	role, err := flow.ParseRole(flagNodeRole)
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse Flow role")
	}

	// create new bucket instance with Flow Bucket name
	bucket := gcs.NewGoogleBucket(flagBucketName)

	// bump up the timeout for an execution node if it has not been explicitly set since downloading
	// root.checkpoint takes a long time
	if role == flow.RoleExecution && !cmd.Flags().Lookup("timeout").Changed {
		flagTimeout = time.Hour
	}

	ctx, cancel := context.WithTimeout(context.Background(), flagTimeout)
	defer cancel()

	// initialize a new client to GCS
	client, err := bucket.NewClient(ctx)
	if err != nil {
		log.Fatal().Err(err).Msgf("error trying get new google bucket client")
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
		fullOutpath := filepath.Join(flagBootDir, "public-root-information", filepath.Base(file))

		log.Info().Str("source", file).Str("dest", fullOutpath).Msgf("downloading file from transit servers")
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

		// root.checkpoint* is downloaded to <bootstrap folder>/public-root-information after a pull
		localPublicRootInfoDir := filepath.Join(flagBootDir, model.DirnamePublicBootstrap)

		// move the root.checkpoint, root.checkpoint.0, root.checkpoint.1 etc. files to the bootstrap/execution-state dir
		err = filepath.WalkDir(localPublicRootInfoDir, func(srcPath string, rootCheckpointFile fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// if rootCheckpointFile is a file whose name starts with "root.checkpoint", then move it
			if !rootCheckpointFile.IsDir() && strings.HasPrefix(rootCheckpointFile.Name(), model.FilenameWALRootCheckpoint) {

				dstPath := filepath.Join(flagBootDir, model.DirnameExecutionState, rootCheckpointFile.Name())
				log.Info().Str("src", srcPath).Str("destination", dstPath).Msgf("moving file")
				err = moveFile(srcPath, dstPath)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to move root.checkpoint files")
		}
	}

	// unwrap consensus node role files
	if role == flow.RoleConsensus {
		err = unWrapFile(flagBootDir, nodeID)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to pull")
		}
	}

	// calculate SHA256 of rootsnapshot
	rootFile := filepath.Join(flagBootDir, model.PathRootProtocolStateSnapshot)
	rootSHA256, err := getFileSHA256(rootFile)
	if err != nil {
		log.Fatal().Err(err).Str("file", rootFile).Msg("failed to calculate SHA256 of root file")
	}
	log.Info().Str("sha256", rootSHA256).Msg("calculated SHA256 of protocol snapshot")
}
