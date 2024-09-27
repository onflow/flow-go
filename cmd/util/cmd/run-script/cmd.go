package run_script

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
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
		payloads, err = util.ReadTrie(flagState, stateCommitment)
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
	ctx := fvm.NewContext(options...)

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

		server, err := rest.NewServer(
			api,
			rest.Config{
				ListenAddress: fmt.Sprintf(":%d", flagPort),
			},
			log.Logger,
			chain,
			metrics.NewNoopCollector(),
			nil,
			backend.Config{},
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
	chainID         flow.ChainID
	vm              *fvm.VirtualMachine
	ctx             fvm.Context
	storageSnapshot registers.StorageSnapshot
}

var _ access.API = &api{}

func (*api) Ping(_ context.Context) error {
	return nil
}

func (a *api) GetNetworkParameters(_ context.Context) access.NetworkParameters {
	return access.NetworkParameters{
		ChainID: a.chainID,
	}
}

func (*api) GetNodeVersionInfo(_ context.Context) (*access.NodeVersionInfo, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetLatestBlockHeader(_ context.Context, _ bool) (*flow.Header, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, errors.New("unimplemented")
}

func (*api) GetBlockHeaderByHeight(_ context.Context, _ uint64) (*flow.Header, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, errors.New("unimplemented")
}

func (*api) GetBlockHeaderByID(_ context.Context, _ flow.Identifier) (*flow.Header, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, errors.New("unimplemented")
}

func (*api) GetLatestBlock(_ context.Context, _ bool) (*flow.Block, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, errors.New("unimplemented")
}

func (*api) GetBlockByHeight(_ context.Context, _ uint64) (*flow.Block, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, errors.New("unimplemented")
}

func (*api) GetBlockByID(_ context.Context, _ flow.Identifier) (*flow.Block, flow.BlockStatus, error) {
	return nil, flow.BlockStatusUnknown, errors.New("unimplemented")
}

func (*api) GetCollectionByID(_ context.Context, _ flow.Identifier) (*flow.LightCollection, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetFullCollectionByID(_ context.Context, _ flow.Identifier) (*flow.Collection, error) {
	return nil, errors.New("unimplemented")
}

func (*api) SendTransaction(_ context.Context, _ *flow.TransactionBody) error {
	return errors.New("unimplemented")
}

func (*api) GetTransaction(_ context.Context, _ flow.Identifier) (*flow.TransactionBody, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetTransactionsByBlockID(_ context.Context, _ flow.Identifier) ([]*flow.TransactionBody, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetTransactionResult(
	_ context.Context,
	_ flow.Identifier,
	_ flow.Identifier,
	_ flow.Identifier,
	_ entities.EventEncodingVersion,
) (*access.TransactionResult, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetTransactionResultByIndex(
	_ context.Context,
	_ flow.Identifier,
	_ uint32,
	_ entities.EventEncodingVersion,
) (*access.TransactionResult, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetTransactionResultsByBlockID(
	_ context.Context,
	_ flow.Identifier,
	_ entities.EventEncodingVersion,
) ([]*access.TransactionResult, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetSystemTransaction(
	_ context.Context,
	_ flow.Identifier,
) (*flow.TransactionBody, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetSystemTransactionResult(
	_ context.Context,
	_ flow.Identifier,
	_ entities.EventEncodingVersion,
) (*access.TransactionResult, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetAccount(_ context.Context, _ flow.Address) (*flow.Account, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetAccountAtLatestBlock(_ context.Context, _ flow.Address) (*flow.Account, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetAccountAtBlockHeight(_ context.Context, _ flow.Address, _ uint64) (*flow.Account, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetAccountBalanceAtLatestBlock(_ context.Context, _ flow.Address) (uint64, error) {
	return 0, errors.New("unimplemented")
}

func (*api) GetAccountBalanceAtBlockHeight(
	_ context.Context,
	_ flow.Address,
	_ uint64,
) (uint64, error) {
	return 0, errors.New("unimplemented")
}

func (*api) GetAccountKeyAtLatestBlock(
	_ context.Context,
	_ flow.Address,
	_ uint32,
) (*flow.AccountPublicKey, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetAccountKeyAtBlockHeight(
	_ context.Context,
	_ flow.Address,
	_ uint32,
	_ uint64,
) (*flow.AccountPublicKey, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetAccountKeysAtLatestBlock(
	_ context.Context,
	_ flow.Address,
) ([]flow.AccountPublicKey, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetAccountKeysAtBlockHeight(
	_ context.Context,
	_ flow.Address,
	_ uint64,
) ([]flow.AccountPublicKey, error) {
	return nil, errors.New("unimplemented")
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

func (*api) ExecuteScriptAtBlockHeight(
	_ context.Context,
	_ uint64,
	_ []byte,
	_ [][]byte,
) ([]byte, error) {
	return nil, errors.New("unimplemented")
}

func (*api) ExecuteScriptAtBlockID(
	_ context.Context,
	_ flow.Identifier,
	_ []byte,
	_ [][]byte,
) ([]byte, error) {
	return nil, errors.New("unimplemented")
}

func (a *api) GetEventsForHeightRange(
	_ context.Context,
	_ string,
	_, _ uint64,
	_ entities.EventEncodingVersion,
) ([]flow.BlockEvents, error) {
	return nil, errors.New("unimplemented")
}

func (a *api) GetEventsForBlockIDs(
	_ context.Context,
	_ string,
	_ []flow.Identifier,
	_ entities.EventEncodingVersion,
) ([]flow.BlockEvents, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetLatestProtocolStateSnapshot(_ context.Context) ([]byte, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetProtocolStateSnapshotByBlockID(_ context.Context, _ flow.Identifier) ([]byte, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetProtocolStateSnapshotByHeight(_ context.Context, _ uint64) ([]byte, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetExecutionResultForBlockID(_ context.Context, _ flow.Identifier) (*flow.ExecutionResult, error) {
	return nil, errors.New("unimplemented")
}

func (*api) GetExecutionResultByID(_ context.Context, _ flow.Identifier) (*flow.ExecutionResult, error) {
	return nil, errors.New("unimplemented")
}

func (*api) SubscribeBlocksFromStartBlockID(
	_ context.Context,
	_ flow.Identifier,
	_ flow.BlockStatus,
) subscription.Subscription {
	return nil
}

func (*api) SubscribeBlocksFromStartHeight(
	_ context.Context,
	_ uint64,
	_ flow.BlockStatus,
) subscription.Subscription {
	return nil
}

func (*api) SubscribeBlocksFromLatest(
	_ context.Context,
	_ flow.BlockStatus,
) subscription.Subscription {
	return nil
}

func (*api) SubscribeBlockHeadersFromStartBlockID(
	_ context.Context,
	_ flow.Identifier,
	_ flow.BlockStatus,
) subscription.Subscription {
	return nil
}

func (*api) SubscribeBlockHeadersFromStartHeight(
	_ context.Context,
	_ uint64,
	_ flow.BlockStatus,
) subscription.Subscription {
	return nil
}

func (*api) SubscribeBlockHeadersFromLatest(
	_ context.Context,
	_ flow.BlockStatus,
) subscription.Subscription {
	return nil
}

func (*api) SubscribeBlockDigestsFromStartBlockID(
	_ context.Context,
	_ flow.Identifier,
	_ flow.BlockStatus,
) subscription.Subscription {
	return nil
}

func (*api) SubscribeBlockDigestsFromStartHeight(
	_ context.Context,
	_ uint64,
	_ flow.BlockStatus,
) subscription.Subscription {
	return nil
}

func (*api) SubscribeBlockDigestsFromLatest(
	_ context.Context,
	_ flow.BlockStatus,
) subscription.Subscription {
	return nil
}

func (*api) SubscribeTransactionStatuses(
	_ context.Context,
	_ *flow.TransactionBody,
	_ entities.EventEncodingVersion,
) subscription.Subscription {
	return nil
}
