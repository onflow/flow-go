package run_script

import (
	"context"
	"fmt"
	"io"
	"os"

	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/engine/access/rest/websockets"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	modutil "github.com/onflow/flow-go/module/util"
)

var (
	flagPayloads        string
	flagState           string
	flagStateCommitment string
	flagChain           string
	flagServe           bool
	flagPort            int
)

var Cmd = &cobra.Command{
	Use:   "run-script",
	Short: "run a script against the execution state",
	Run:   run,
}

func init() {

	// Input 1

	Cmd.Flags().StringVar(
		&flagPayloads,
		"payloads",
		"",
		"Input payload file name",
	)

	Cmd.Flags().StringVar(
		&flagState,
		"state",
		"",
		"Input state file name",
	)
	Cmd.Flags().StringVar(
		&flagStateCommitment,
		"state-commitment",
		"",
		"Input state commitment",
	)

	Cmd.Flags().StringVar(
		&flagChain,
		"chain",
		"",
		"Chain name",
	)
	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().BoolVar(
		&flagServe,
		"serve",
		false,
		"serve with an HTTP server",
	)

	Cmd.Flags().IntVar(
		&flagPort,
		"port",
		8000,
		"port for HTTP server",
	)
}

func run(*cobra.Command, []string) {

	if flagPayloads == "" && flagState == "" {
		log.Fatal().Msg("Either --payloads or --state must be provided")
	} else if flagPayloads != "" && flagState != "" {
		log.Fatal().Msg("Only one of --payloads or --state must be provided")
	}
	if flagState != "" && flagStateCommitment == "" {
		log.Fatal().Msg("--state-commitment must be provided when --state is provided")
	}

	chainID := flow.ChainID(flagChain)
	// Validate chain ID
	chain := chainID.Chain()

	log.Info().Msg("loading state ...")

	var (
		err      error
		payloads []*ledger.Payload
	)
	if flagPayloads != "" {
		_, payloads, err = util.ReadPayloadFile(log.Logger, flagPayloads)
	} else {
		log.Info().Msg("reading trie")

		stateCommitment := util.ParseStateCommitment(flagStateCommitment)
		payloads, err = util.ReadTrieForPayloads(flagState, stateCommitment)
	}
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read payloads")
	}

	log.Info().Msgf("creating registers from payloads (%d)", len(payloads))

	registersByAccount, err := registers.NewByAccountFromPayloads(payloads)
	if err != nil {
		log.Fatal().Err(err)
	}
	log.Info().Msgf(
		"created %d registers from payloads (%d accounts)",
		registersByAccount.Count(),
		registersByAccount.AccountCount(),
	)

	options := computation.DefaultFVMOptions(chainID, false, false)
	options = append(
		options,
		fvm.WithContractDeploymentRestricted(false),
		fvm.WithContractRemovalRestricted(false),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
		fvm.WithTransactionFeesEnabled(false),
	)
	ctx := fvm.NewContext(chain, options...)

	storageSnapshot := registers.StorageSnapshot{
		Registers: registersByAccount,
	}

	vm := fvm.NewVirtualMachine()

	if flagServe {
		api := &api{
			chainID:         chainID,
			vm:              vm,
			ctx:             ctx,
			storageSnapshot: storageSnapshot,
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		irrCtx, errCh := irrecoverable.WithSignaler(ctx)
		go func() {
			err := modutil.WaitError(errCh, ctx.Done())
			if err != nil {
				log.Fatal().Err(err).Msg("server finished with error")
			}
		}()

		server, err := rest.NewServer(
			irrCtx,
			api,
			rest.Config{
				ListenAddress: fmt.Sprintf(":%d", flagPort),
			},
			log.Logger,
			chain,
			metrics.NewNoopCollector(),
			nil,
			backend.Config{},
			false,
			websockets.NewDefaultWebsocketConfig(),
			nil,
		)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create server")
		}

		log.Info().Msgf("serving on port %d", flagPort)

		err = server.ListenAndServe()
		if err != nil {
			log.Info().Msg("server stopped")
		}
	} else {
		code, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal().Msgf("failed to read script: %s", err)
		}

		encodedResult, err := runScript(vm, ctx, storageSnapshot, code, nil)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to run script")
		}

		_, _ = os.Stdout.Write(encodedResult)
	}
}

func runScript(
	vm *fvm.VirtualMachine,
	ctx fvm.Context,
	storageSnapshot snapshot.StorageSnapshot,
	code []byte,
	arguments [][]byte,
) (
	encodedResult []byte,
	err error,
) {
	_, res, err := vm.Run(
		ctx,
		fvm.Script(code).WithArguments(arguments...),
		storageSnapshot,
	)
	if err != nil {
		return nil, err
	}

	if res.Err != nil {
		return nil, res.Err
	}

	encoded, err := jsoncdc.Encode(res.Value)
	if err != nil {
		return nil, err
	}

	return encoded, nil
}

type api struct {
	access.UnimplementedAPI
	chainID         flow.ChainID
	vm              *fvm.VirtualMachine
	ctx             fvm.Context
	storageSnapshot registers.StorageSnapshot
}

var _ access.API = &api{}

func (*api) Ping(_ context.Context) error {
	return nil
}

func (a *api) GetNetworkParameters(_ context.Context) accessmodel.NetworkParameters {
	return accessmodel.NetworkParameters{
		ChainID: a.chainID,
	}
}

func (a *api) ExecuteScriptAtLatestBlock(
	_ context.Context,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	return runScript(
		a.vm,
		a.ctx,
		a.storageSnapshot,
		script,
		arguments,
	)
}
