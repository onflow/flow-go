package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go-sdk/client"

	model "github.com/onflow/flow-go/model/bootstrap"
)

var flagAccessAddress string

// snapshotCmd represents the command to download the latest protocol snapshot to bootstrap from
var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Downloads the latest serialized protocol snapshot from an access node to be used to bootstrap from",
	Long:  `Downloads the latest serialized protocol snapshot from an access node to be used to bootstrap from`,
	Run:   snapshot,
}

func init() {
	rootCmd.AddCommand(snapshotCmd)

	// required parameters
	snapshotCmd.Flags().StringVar(&flagAccessAddress, "access-address", "", "the address of an access node")
	_ = snapshotCmd.MarkFlagRequired("config")
}

func snapshot(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	flowClient, err := client.New(flagAccessAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Msgf("could not create flow client: %v", err)
	}

	bytes, err := flowClient.GetLatestProtocolStateSnapshot(ctx)
	if err != nil {
		log.Fatal().Msg("could not get latest protocol snapshot from access node")
	}

	writeText(model.PathRootProtocolStateSnapshot, bytes)
}
