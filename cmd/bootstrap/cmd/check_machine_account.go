package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go/cmd"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/module/epochs"
)

var (
	flagAccessAPIAddress string
)

var checkMachineAccountCmd = &cobra.Command{
	Use:   "check-machine-account",
	Short: "checks a machine account configuration",
	Run:   checkMachineAccountRun,
}

func init() {
	rootCmd.AddCommand(checkMachineAccountCmd)

	checkMachineAccountCmd.Flags().StringVar(&flagAccessAPIAddress, "access-address", "", "network address of an Access Node")
	cmd.MarkFlagRequired(checkMachineAccountCmd, "access-address")
}

func checkMachineAccountRun(_ *cobra.Command, _ []string) {

	// read nodeID written to boostrap dir by `bootstrap key`
	nodeID, err := readNodeID()
	if err != nil {
		log.Fatal().Err(err).Msg("could not read node id")
	}

	// read the private node information - used to get the role
	var nodeInfoPriv model.NodeInfoPriv
	readJSON(filepath.Join(flagOutdir, fmt.Sprintf(model.PathNodeInfoPriv, nodeID)), &nodeInfoPriv)

	// read the machine account info file
	machineAccountInfo := readMachineAccountInfo(nodeID)

	machineAccountPrivKey, err := machineAccountInfo.PrivateKey()
	if err != nil {
		log.Fatal().Err(err).Msg("could not decode machine account private key")
	}

	// print public machine account info
	log.Debug().
		Str("machine_account_address", machineAccountInfo.Address).
		Str("machine_account_pub_key", fmt.Sprintf("%x", encodedRuntimeAccountPubKey(machineAccountPrivKey))).
		Uint("key_index", machineAccountInfo.KeyIndex).
		Str("signing_algo", machineAccountInfo.SigningAlgorithm.String()).
		Str("hash_algo", machineAccountInfo.HashAlgorithm.String()).
		Msg("read machine account info from disk")

	flowClient, err := client.New(flagAccessAPIAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Err(err).Msgf("could not connect to access API at address %s", flagAccessAPIAddress)
	}

	// retrieve the on-chain machine account info
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	onChainAccount, err := flowClient.GetAccount(ctx, sdk.HexToAddress(machineAccountInfo.Address))
	if err != nil {
		log.Fatal().Err(err).Msg("could not read account")
	}

	// check the local machine account config with the on-chain account
	// this will log non-critical warnings, and return an error for critical problems
	err = epochs.CheckMachineAccountInfo(
		log,
		nodeInfoPriv.Role,
		machineAccountInfo,
		onChainAccount,
	)
	if err != nil {
		log.Error().Err(err).Msg("⚠️ machine account is misconfigured")
		return
	}
	log.Info().Msg("🤖 machine account is configured correctly")
}

// readMachineAccountInfo reads the machine account info from disk
func readMachineAccountInfo(nodeID string) model.NodeMachineAccountInfo {
	var machineAccountInfo model.NodeMachineAccountInfo

	path := filepath.Join(flagOutdir, fmt.Sprintf(model.PathNodeMachineAccountInfoPriv, nodeID))
	readJSON(path, &machineAccountInfo)

	return machineAccountInfo
}
