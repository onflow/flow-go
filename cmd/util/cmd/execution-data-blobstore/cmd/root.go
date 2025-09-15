package cmd

import (
	"fmt"
	"os"

	"github.com/ipfs/go-datastore"
	pebbleds "github.com/ipfs/go-ds-pebble"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/onflow/flow-go/module/blobs"
)

var (
	flagBlobstoreDir string
)

var rootCmd = &cobra.Command{
	Use:   "execution-data-blobstore",
	Short: "interact with execution data blobstore",
}

var RootCmd = rootCmd

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
}

func initBlobstore() (blobs.Blobstore, datastore.Batching) {
	ds, err := pebbleds.NewDatastore(flagBlobstoreDir, nil)

	if err != nil {
		log.Fatal().Err(err).Msg("could not init pebble datastore")
	}

	blobstore := blobs.NewBlobstore(ds)

	return blobstore, ds
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagBlobstoreDir, "blobstore-dir", "d", "./execution_data_blobstore", "directory to the execution data blobstore")
	_ = rootCmd.MarkPersistentFlagRequired("blobstore-dir")

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()
}
