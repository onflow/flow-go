package debug_tx

import (
	"context"
	"os"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/onflow/flow/protobuf/go/flow/executiondata"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
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
	_ = Cmd.MarkFlagRequired("execution-address")

	Cmd.Flags().StringVar(&flagBlockID, "block-id", "", "block ID")
	_ = Cmd.MarkFlagRequired("block-id")

	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().StringVar(&flagScript, "script", "", "path to script")
	_ = Cmd.MarkFlagRequired("script")

	Cmd.Flags().BoolVar(&flagUseExecutionDataAPI, "use-execution-data-api", false, "use the execution data API")
}

func run(*cobra.Command, []string) {

	chainID := flow.ChainID(flagChain)
	chain := chainID.Chain()

	code, err := os.ReadFile(flagScript)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to read script from file %s", flagScript)
	}

	log.Info().Msg("Fetching block header ...")

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

	header, err := debug.GetAccessAPIBlockHeader(accessClient, context.Background(), blockID)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to fetch block header")
	}

	blockHeight := header.Height

	log.Info().Msgf(
		"Fetched block header: %s (height %d)",
		header.ID(),
		blockHeight,
	)

	var snap snapshot.StorageSnapshot

	if flagUseExecutionDataAPI {
		executionDataClient := executiondata.NewExecutionDataAPIClient(accessConn)
		snap, err = debug.NewExecutionDataStorageSnapshot(executionDataClient, nil, blockHeight)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create storage snapshot")
		}
	} else {
		executionConn, err := grpc.NewClient(
			flagExecutionAddress,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create execution connection")
		}
		defer executionConn.Close()

		executionClient := execution.NewExecutionAPIClient(executionConn)
		snap, err = debug.NewExecutionNodeStorageSnapshot(executionClient, nil, blockID)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create storage snapshot")
		}
	}

	debugger := debug.NewRemoteDebugger(chain, log.Logger)

	// TODO: add support for arguments
	var arguments [][]byte

	result, scriptErr, processErr := debugger.RunScript(code, arguments, snap, header)

	if scriptErr != nil {
		log.Fatal().Err(scriptErr).Msg("transaction error")
	}
	if processErr != nil {
		log.Fatal().Err(processErr).Msg("process error")
	}
	log.Info().Msgf("result: %s", result)
}
