package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/onflow/flow-go/engine/common/synchronization"

	"github.com/onflow/cadence/runtime"
	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/provider"
	"github.com/onflow/flow-go/engine/common/requester"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/ingestion"
	exeprovider "github.com/onflow/flow-go/engine/execution/provider"
	"github.com/onflow/flow-go/engine/execution/rpc"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/engine/execution/sync"
	"github.com/onflow/flow-go/fvm"
	ledger "github.com/onflow/flow-go/ledger/complete"
	wal "github.com/onflow/flow-go/ledger/complete/wal"
	bootstrapFilenames "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	chainsync "github.com/onflow/flow-go/module/synchronization"
	storage "github.com/onflow/flow-go/storage/badger"
)

func main() {

	var (
		ledgerStorage         *ledger.Ledger
		events                *storage.Events
		txResults             *storage.TransactionResults
		results               *storage.ExecutionResults
		receipts              *storage.ExecutionReceipts
		providerEngine        *exeprovider.Engine
		syncCore              *chainsync.Core
		syncEngine            *synchronization.Engine
		computationManager    *computation.Manager
		collectionRequester   *requester.Engine
		ingestionEng          *ingestion.Engine
		rpcConf               rpc.Config
		err                   error
		executionState        state.ExecutionState
		triedir               string
		collector             module.ExecutionMetrics
		mTrieCacheSize        uint32
		checkpointDistance    uint
		requestInterval       time.Duration
		preferredExeNodeIDStr string
		syncByBlocks          bool
	)

	cmd.FlowNode(flow.RoleExecution.String()).
		ExtraFlags(func(flags *pflag.FlagSet) {
			homedir, _ := os.UserHomeDir()
			datadir := filepath.Join(homedir, ".flow", "execution")

			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "i", "localhost:9000", "the address the gRPC server listens on")
			flags.StringVar(&triedir, "triedir", datadir, "directory to store the execution State")
			flags.Uint32Var(&mTrieCacheSize, "mtrie-cache-size", 1000, "cache size for MTrie")
			flags.UintVar(&checkpointDistance, "checkpoint-distance", 1, "number of WAL segments between checkpoints")
			flags.DurationVar(&requestInterval, "request-interval", 60*time.Second, "the interval between requests for the requester engine")
			flags.StringVar(&preferredExeNodeIDStr, "preferred-exe-node-id", "", "node ID for preferred execution node used for state sync")
			flags.BoolVar(&syncByBlocks, "sync-by-blocks", true, "sync by blocks instead of execution state deltas")
		}).
		Module("computation manager", func(node *cmd.FlowNodeBuilder) error {
			rt := runtime.NewInterpreterRuntime()

			vm := fvm.New(rt)

			vmCtx := fvm.NewContext(
				fvm.WithChain(node.RootChainID.Chain()),
				fvm.WithBlocks(node.Storage.Blocks),
			)

			manager, err := computation.New(
				node.Logger,
				collector,
				node.Tracer,
				node.Me,
				node.State,
				vm,
				vmCtx,
			)
			computationManager = manager

			return err
		}).
		Module("execution metrics", func(node *cmd.FlowNodeBuilder) error {
			collector = metrics.NewExecutionCollector(node.Tracer, node.MetricsRegisterer)
			return nil
		}).
		Module("sync core", func(node *cmd.FlowNodeBuilder) error {
			syncCore, err = chainsync.New(node.Logger, chainsync.DefaultConfig())
			return err
		}).
		Module("execution receipts storage", func(node *cmd.FlowNodeBuilder) error {
			results = storage.NewExecutionResults(node.DB)
			receipts = storage.NewExecutionReceipts(node.DB, results)
			return nil
		}).
		Component("execution state ledger", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			// check if the execution database already exists
			bootstrapper := bootstrap.NewBootstrapper(node.Logger)

			commit, bootstrapped, err := bootstrapper.IsBootstrapped(node.DB)
			if err != nil {
				return nil, fmt.Errorf("could not query database to know whether database has been bootstrapped: %w", err)
			}

			// if the execution database does not exist, then we need to bootstrap the execution database.
			if !bootstrapped {
				// when bootstrapping, the bootstrap folder must have a checkpoint file
				// we need to cover this file to the trie folder to restore the trie to restore the execution state.
				err = copyBootstrapState(node.BaseConfig.BootstrapDir, triedir)
				if err != nil {
					return nil, fmt.Errorf("could not load bootstrap state from checkpoint file: %w", err)
				}

				// TODO: check that the checkpoint file contains the root block's statecommit hash

				err = bootstrapper.BootstrapExecutionDatabase(node.DB, node.RootSeal.FinalState, node.RootBlock.Header)
				if err != nil {
					return nil, fmt.Errorf("could not bootstrap execution database: %w", err)
				}
			} else {
				// if execution database has been bootstrapped, then the root statecommit must equal to the one
				// in the bootstrap folder
				if !bytes.Equal(commit, node.RootSeal.FinalState) {
					return nil, fmt.Errorf("mismatching root statecommitment. database has state commitment: %v, "+
						"bootstap has statecommitment: %v",
						commit, node.RootSeal.FinalState)
				}
			}

			ledgerStorage, err = ledger.NewLedger(triedir, int(mTrieCacheSize), collector, node.Logger.With().Str("subcomponent", "ledger").Logger(), node.MetricsRegisterer)
			return ledgerStorage, err
		}).
		Component("execution state ledger WAL compactor", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			checkpointer, err := ledgerStorage.Checkpointer()
			if err != nil {
				return nil, fmt.Errorf("cannot create checkpointer: %w", err)
			}
			compactor := wal.NewCompactor(checkpointer, 10*time.Second, checkpointDistance)

			return compactor, nil
		}).
		Component("provider engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			chunkDataPacks := storage.NewChunkDataPacks(node.DB)
			stateCommitments := storage.NewCommits(node.Metrics.Cache, node.DB)

			executionState = state.NewExecutionState(
				ledgerStorage,
				stateCommitments,
				node.Storage.Blocks,
				node.Storage.Collections,
				chunkDataPacks,
				results,
				receipts,
				node.DB,
				node.Tracer,
			)

			stateSync := sync.NewStateSynchronizer(executionState)

			providerEngine, err = exeprovider.New(
				node.Logger,
				node.Tracer,
				node.Network,
				node.State,
				node.Me,
				executionState,
				stateSync,
				collector,
			)

			return providerEngine, err
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			collectionRequester, err = requester.New(node.Logger, node.Metrics.Engine, node.Network, node.Me, node.State,
				engine.RequestCollections,
				filter.HasRole(flow.RoleCollection),
				func() flow.Entity { return &flow.Collection{} },
				// we are manually triggering batches in execution, but lets still send off a batch once a minute, as a safety net for the sake of retries
				requester.WithBatchInterval(requestInterval),
			)

			preferredExeFilter := filter.Any
			preferredExeNodeID, err := flow.HexStringToIdentifier(preferredExeNodeIDStr)
			if err == nil {
				node.Logger.Info().Hex("prefered_exe_node_id", preferredExeNodeID[:]).Msg("starting with preferred exe sync node")
				preferredExeFilter = filter.HasNodeID(preferredExeNodeID)
			} else if err != nil && preferredExeNodeIDStr != "" {
				node.Logger.Debug().Str("prefered_exe_node_id_string", preferredExeNodeIDStr).Msg("could not parse exe node id, starting WITHOUT preferred exe sync node")
			}

			// Needed for gRPC server, make sure to assign to main scoped vars
			events = storage.NewEvents(node.DB)
			txResults = storage.NewTransactionResults(node.DB)
			ingestionEng, err = ingestion.New(
				node.Logger,
				node.Network,
				node.Me,
				collectionRequester,
				node.State,
				node.Storage.Blocks,
				node.Storage.Payloads,
				node.Storage.Collections,
				events,
				txResults,
				computationManager,
				providerEngine,
				syncCore,
				executionState,
				6, // TODO - config param maybe?
				preferredExeFilter,
				syncByBlocks,
				collector,
				node.Tracer,
				true,
			)

			// TODO: we should solve these mutual dependencies better
			// => https://github.com/dapperlabs/flow-go/issues/4360
			collectionRequester = collectionRequester.WithHandle(ingestionEng.OnCollection)

			return ingestionEng, err
		}).
		Component("collection requester engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			// We initialize the requester engine inside the ingestion engine due to the mutual dependency. However, in
			// order for it to properly start and shut down, we should still return it as its own engine here, so it can
			// be handled by the scaffold.
			return collectionRequester, nil
		}).
		Component("receipt provider engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			retrieve := func(blockID flow.Identifier) (flow.Entity, error) {
				return receipts.ByBlockID(blockID)
			}
			eng, err := provider.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.State,
				engine.ProvideReceiptsByBlockID,
				filter.HasRole(flow.RoleConsensus),
				retrieve,
			)
			return eng, err
		}).
		// TODO: currently issues with this engine on the EXE node, as there is no follower engine, https://github.com/dapperlabs/flow-go/issues/4382
		Component("sychronization engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			// initialize the synchronization engine
			syncEngine, err = synchronization.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.State,
				node.Storage.Blocks,
				ingestionEng,
				syncCore,
				// Right now, only used to sync, so never poll
				synchronization.WithPollInterval(time.Duration(0)),
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize synchronization engine: %w", err)
			}

			return syncEngine, nil
		}).
		Component("grpc server", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rpcEng := rpc.New(node.Logger, rpcConf, ingestionEng, node.Storage.Blocks, events, results, txResults, node.RootChainID)
			return rpcEng, nil
		}).Run()
}

// copy the checkpoint files from the bootstrap folder to the execution state folder
// Checkpoint file is required to restore the trie, and has to be placed in the execution
// state folder.
// There are two ways to generate a checkpoint file:
// 1) From a clean state.
// 		Refer to the code in the testcase: TestGenerateExecutionState
// 2) From a previous execution state
// 		This is often used when sporking the network.
//    Use the execution-state-extract util commandline to generate a checkpoint file from
// 		a previous checkpoint file
func copyBootstrapState(dir, trie string) error {
	filename := ""
	firstCheckpointFilename := "00000000"

	fileExists := func(fileName string) bool {
		_, err := os.Stat(filepath.Join(dir, bootstrapFilenames.DirnameExecutionState, fileName))
		return err == nil
	}

	// if there is a root checkpoint file, then copy that file over
	if fileExists(wal.RootCheckpointFilename) {
		filename = wal.RootCheckpointFilename
	} else if fileExists(firstCheckpointFilename) {
		// else if there is a checkpoint file, then copy that file over
		filename = firstCheckpointFilename
	} else {
		filePath := filepath.Join(dir, bootstrapFilenames.DirnameExecutionState, firstCheckpointFilename)

		// include absolute path of the missing file in the error message
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			absPath = filePath
		}

		return fmt.Errorf("execution state file not found: %v", absPath)
	}

	// copy from the bootstrap folder to the execution state folder
	src := filepath.Join(dir, bootstrapFilenames.DirnameExecutionState, filename)
	dst := filepath.Join(trie, filename)

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// It's possible that the trie dir does not yet exist. If not this will create the the required path
	err = os.MkdirAll(trie, 0700)
	if err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	fmt.Printf("copied bootstrap state file from: %v, to: %v\n", src, dst)

	return out.Close()
}
