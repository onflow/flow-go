package debug_tx

import (
	"context"
	"os"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/debug"
)

// use the following command to forward port 9000 from the EN to localhost:9001
// `gcloud compute ssh '--ssh-flag=-A' --no-user-output-enabled --tunnel-through-iap migrationmainnet1-execution-001 --project flow-multi-region -- -NL 9001:localhost:9000`

var (
	flagAccessAddress       string
	flagExecutionAddress    string
	flagBlockID             string
	flagChain               string
	flagScript              string
	flagUseExecutionDataAPI bool
	flagUseVM               bool
)

var Cmd = &cobra.Command{
	Use:   "debug-script",
	Short: "debug a script",
	Run:   run,
}

func init() {

	Cmd.Flags().StringVar(
		&flagChain,
		"chain",
		"",
		"Chain name",
	)

	Cmd.Flags().StringVar(&flagAccessAddress, "access-address", "", "address of the access node")
	_ = Cmd.MarkFlagRequired("access-address")

	Cmd.Flags().StringVar(&flagExecutionAddress, "execution-address", "", "address of the execution node")

	Cmd.Flags().StringVar(&flagBlockID, "block-id", "", "block ID")
	_ = Cmd.MarkFlagRequired("block-id")

	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().StringVar(&flagScript, "script", "", "path to script")
	_ = Cmd.MarkFlagRequired("script")

	Cmd.Flags().BoolVar(&flagUseExecutionDataAPI, "use-execution-data-api", true, "use the execution data API (default: true)")

	Cmd.Flags().BoolVar(&flagUseVM, "use-vm", false, "use the VM for script execution (default: false)")
}

func run(*cobra.Command, []string) {

	chainID := flow.ChainID(flagChain)
	chain := chainID.Chain()

	code, err := os.ReadFile(flagScript)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to read script from file %s", flagScript)
	}

	accessConn, err := grpc.NewClient(
		flagAccessAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create access connection")
	}
	defer accessConn.Close()

	accessClient := access.NewAccessAPIClient(accessConn)

	blockID, err := flow.HexStringToIdentifier(flagBlockID)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse block ID")
	}

	log.Info().Msgf("Fetching block header for %s ...", blockID)

	blockHeader, err := debug.GetAccessAPIBlockHeader(context.Background(), accessClient, blockID)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to fetch block header")
	}

	log.Info().Msgf(
		"Fetched block header: %s (height %d)",
		blockID,
		blockHeader.Height,
	)

	var remoteClient debug.RemoteClient
	if flagUseExecutionDataAPI {
		remoteClient, err = debug.NewExecutionDataRemoteClient(flagAccessAddress, chain)
	} else if flagExecutionAddress != "" {
		remoteClient, err = debug.NewExecutionNodeRemoteClient(flagExecutionAddress)
	} else {
		log.Fatal().Msg("either --use-execution-data-api or --execution-address must be provided")
	}
	if err != nil {
		log.Fatal().Err(err).Msg("failed to remote client")
	}
	defer remoteClient.Close()

	remoteSnapshot, err := remoteClient.StorageSnapshot(blockHeader.Height, blockID)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create storage snapshot")
	}

	blockSnapshot := debug.NewCachingStorageSnapshot(remoteSnapshot)

	debugger := debug.NewRemoteDebugger(chain, log.Logger, flagUseVM, flagUseVM)

	// TODO: add support for arguments
	var arguments [][]byte

	result, err := debugger.RunScript(code, arguments, blockSnapshot, blockHeader)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to run script")
	}

	if result.Output.Err != nil {
		log.Fatal().Err(result.Output.Err).Msg("script execution failed")
	}

	log.Info().Msgf("Result: %s", result.Output.Value)
}
