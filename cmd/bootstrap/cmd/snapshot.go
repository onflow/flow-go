package cmd

import (
	"context"
	"encoding/json"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go-sdk/client"

	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

var flagAccessAddress string
var flagNodeID string

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
	_ = snapshotCmd.MarkFlagRequired("access-address")
	snapshotCmd.Flags().StringVar(&flagNodeID, "node-id", "", "")
	_ = snapshotCmd.MarkFlagRequired("access-address")
}

func snapshot(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	// convert the nodeID string to `flow.Identifier`
	nodeID, err := flow.HexStringToIdentifier(flagNodeID)
	if err != nil {
		log.Fatal().Msg("could not convert node id to identifier")
	}

	// create a flow client with given access address
	flowClient, err := client.New(flagAccessAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Msgf("could not create flow client: %v", err)
	}

	// get latest snapshot bytes encoded as JSON
	bytes, err := flowClient.GetLatestProtocolStateSnapshot(ctx)
	if err != nil {
		log.Fatal().Msg("could not get latest protocol snapshot from access node")
	}

	// unmarshal bytes to snapshot object
	var snapshot inmem.Snapshot
	err = json.Unmarshal(bytes, &snapshot)
	if err != nil {
		log.Fatal().Msg("could not unmarshal snapshot data retreived from access node")
	}

	currentIdentities, err := snapshot.Epochs().Current().InitialIdentities()
	if err != nil {
		log.Fatal().Msg("could not get initial identities from current epoch")
	}

	// check if in correct phase of epoch to write snapshot
	if ok := checkEpochPhase(snapshot); !ok {
		return
	}

	if _, ok := currentIdentities.ByNodeID(nodeID); !ok {
		identities, err := snapshot.Epochs().Next().InitialIdentities()
		if err != nil {
			log.Fatal().Msg("could not get initial identities from next epoch")
		}

		if _, ok := identities.ByNodeID(nodeID); !ok {
			log.Error().Msgf("given node id does not belong in the current epoch")
			return
		}
	}

	writeText(model.PathRootProtocolStateSnapshot, bytes)
}

// checkEpochPhase ensures that the given snapshot is part of the EpochSetup or the
// EpochCommitted and not the EpochStaking phase
func checkEpochPhase(snapshot inmem.Snapshot) bool {
	phase, err := snapshot.Phase()
	if err != nil {
		log.Error().Msgf("could not get phase from snapshot")
		return false
	}

	// snapshot should not be for staking, must be setup or committed phase
	if phase == flow.EpochPhaseStaking {
		log.Error().Msg("please request snapshot after staking phase has ended")
		return false
	}

	return true
}
