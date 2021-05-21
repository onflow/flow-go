package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"github.com/onflow/cadence"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"

	"github.com/onflow/flow-go/crypto"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	ioutils "github.com/onflow/flow-go/utils/io"
)

var (
	flagAccessAddress string
)

// machineAccountCmd represents the `machine-account` command which generates required machine account file
// for existing and new operators. New operators would have run the `bootstrap keys` cmd get all three keys
// before running this command.
var machineAccountCmd = &cobra.Command{
	Use:   "machine-account",
	Short: "",
	Run:   machineAccountRun,
}

func init() {
	rootCmd.AddCommand(machineAccountCmd)

	machineAccountCmd.Flags().StringVar(&flagMachineAccountAddress, "address", "", "the node's machine account address")
	_ = machineAccountCmd.MarkFlagRequired("address")

	machineAccountCmd.Flags().StringVar(&flagAccessAddress, "access-address", "", "the network address of an access node")
	_ = machineAccountCmd.MarkFlagRequired("access-address")
}

// keyCmdRun generate the node staking key, networking key and node information
func machineAccountRun(_ *cobra.Command, _ []string) {

	// read nodeID written to boostrap dir by `bootstrap key`
	nodeID, err := readNodeID()
	if err != nil {
		log.Fatal().Err(err).Msg("could not read node id")
	}

	// check if node-machine-account-key.priv.json path exists
	machineAccountKeyPath := fmt.Sprintf(model.PathNodeMachineAccountPrivateKey, nodeID)
	keyExists, err := pathExists(filepath.Join(flagOutdir, machineAccountKeyPath))
	if err != nil {
		log.Fatal().Err(err).Msg("could not check if node-machine-account-key.priv.json exists")
	}
	if !keyExists {
		log.Fatal().Msg("could not read machine account private key file: run `bootstrap machine-account-key` to create one")
	}

	// check if node-machine-account-info.priv.json file exists in boostrap dir
	machineAccountInfoPath := fmt.Sprintf(model.PathNodeMachineAccountInfoPriv, nodeID)
	infoExists, err := pathExists(filepath.Join(flagOutdir, machineAccountInfoPath))
	if err != nil {
		log.Fatal().Err(err).Msg("could not check if node-machine-account-info.priv.json exists")
	}
	if infoExists {
		log.Info().Str("path", machineAccountInfoPath).Msg("node maching account info file already exists")
		return
	}

	// read in machine account private key
	machinePrivKey := readMachineAccountPriv(nodeID)
	log.Info().Msg("read machine account private key json")

	// create account on access node
	accountAddress, err := createAccount(machinePrivKey)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create account for private key")
	}

	// create node-machine-account-info.priv.json file
	machineAccountInfo := assembleNodeMachineAccountInfo(machineAccountAddress, 0, machinePrivKey)

	// write machine account info
	writeJSON(fmt.Sprintf(model.PathNodeMachineAccountInfoPriv, nodeID), machineAccountInfo)
}

func createAccount(privateKey crypto.PrivateKey) (sdk.Address, error) {
	accessClient, err := client.New(flagAccessAddress, grpc.WithInsecure())
	if err != nil {
		return sdk.Address{}, fmt.Errorf("could not initialize client to access-address: %w", err)
	}

	ctx := context.Background()

	// get latest block
	latestBlock, err := accessClient.GetLatestBlock(ctx, true)
	if err != nil {
		return sdk.Address{}, fmt.Errorf("could not get latest block from access node: %w", err)
	}

	// create account for private key
	accountKey := sdk.NewAccountKey().FromPrivateKey(privateKey).
		SetSigAlgo(sdkcrypto.ECDSA_P256).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)
	accountSigner := sdkcrypto.NewInMemorySigner(privateKey, accountKey.HashAlgo)

	// TODO: need to define proposer and payer for tx
	createAccountTx := templates.CreateAccount([]*sdk.AccountKey{accountKey}, []templates.Contract{}, sdk.Address{})
	createAccountTx.SetGasLimit(flow.DefaultMaxTransactionGasLimit).
		SetReferenceBlockID(sdk.Identifier(latestBlock.ID)).
		SetProposalKey(sdk.Address{}, 0, 0).
		SetPayer(sdk.Address{})

	// sign transaction
	err = createAccountTx.SignEnvelope(sdk.Address{}, 0, accountSigner)
	if err != nil {
		return sdk.Address{}, fmt.Errorf("could not sign transaction: %w", err)
	}

	// submit transaction to create account
	err = accessClient.SendTransaction(ctx, *createAccountTx)
	if err != nil {
		return sdk.Address{}, fmt.Errorf("could not submit create account transaction: %w", err)
	}

	// get transaction result
	txResult, err := accessClient.GetTransactionResult(ctx, createAccountTx.ID())
	if err != nil {
		return sdk.Address{}, fmt.Errorf("could not get transaction result for create account tx: %w", err)
	}

	// find account created event with account address
	var address sdk.Address
	for _, event := range txResult.Events {
		if event.Type == sdk.EventAccountCreated {
			address = sdk.Address(event.Value.Fields[0].(cadence.Address))
			break
		}
	}

	if address == (sdk.Address{}) {
		return sdk.Address{}, fmt.Errorf("failed to find AccountCreated event")
	}

	return address, nil
}

// readMachineAccountPriv reads the machine account private key files in the bootstrap dir
func readMachineAccountPriv(nodeID string) crypto.PrivateKey {
	var machineAccountPriv model.NodeMachineAccountPriv

	path := filepath.Join(flagOutdir, fmt.Sprintf(model.PathNodeMachineAccountPrivateKey, nodeID))
	readJSON(path, &machineAccountPriv)

	return machineAccountPriv.PrivateKey.PrivateKey
}

// readNodeID reads the NodeID file written by `bootstrap key` command
func readNodeID() (string, error) {
	path := filepath.Join(flagOutdir, model.PathNodeID)

	data, err := ioutils.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("error reading file %s: %w", path, err)
	}

	return strings.TrimSpace(string(data)), nil
}
