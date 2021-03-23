package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/onflow/cadence/runtime"
	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/engine"
	followereng "github.com/onflow/flow-go/engine/common/follower"
	"github.com/onflow/flow-go/engine/common/provider"
	"github.com/onflow/flow-go/engine/common/requester"
	"github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/engine/execution/checker"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/ingestion"
	exeprovider "github.com/onflow/flow-go/engine/execution/provider"
	"github.com/onflow/flow-go/engine/execution/rpc"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/extralog"
	ledger "github.com/onflow/flow-go/ledger/complete"
	wal "github.com/onflow/flow-go/ledger/complete/wal"
	bootstrapFilenames "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/signature"
	chainsync "github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	storage "github.com/onflow/flow-go/storage/badger"
)

func main() {

	var (
		followerState               protocol.MutableState
		ledgerStorage               *ledger.Ledger
		events                      *storage.Events
		serviceEvents               *storage.ServiceEvents
		txResults                   *storage.TransactionResults
		results                     *storage.ExecutionResults
		receipts                    *storage.ExecutionReceipts
		myReceipts                  *storage.MyExecutionReceipts
		providerEngine              *exeprovider.Engine
		checkerEng                  *checker.Engine
		syncCore                    *chainsync.Core
		pendingBlocks               *buffer.PendingBlocks // used in follower engine
		deltas                      *ingestion.Deltas
		syncEngine                  *synchronization.Engine
		followerEng                 *followereng.Engine // to sync blocks from consensus nodes
		computationManager          *computation.Manager
		collectionRequester         *requester.Engine
		ingestionEng                *ingestion.Engine
		rpcConf                     rpc.Config
		err                         error
		executionState              state.ExecutionState
		triedir                     string
		collector                   module.ExecutionMetrics
		mTrieCacheSize              uint32
		transactionResultsCacheSize uint
		checkpointDistance          uint
		checkpointsToKeep           uint
		stateDeltasLimit            uint
		cadenceExecutionCache       uint
		requestInterval             time.Duration
		preferredExeNodeIDStr       string
		syncByBlocks                bool
		syncFast                    bool
		syncThreshold               int
		extensiveLog                bool
		checkStakedAtBlock          func(blockID flow.Identifier) (bool, error)
	)

	cmd.FlowNode(flow.RoleExecution.String()).
		ExtraFlags(func(flags *pflag.FlagSet) {
			homedir, _ := os.UserHomeDir()
			datadir := filepath.Join(homedir, ".flow", "execution")

			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "i", "localhost:9000", "the address the gRPC server listens on")
			flags.StringVar(&triedir, "triedir", datadir, "directory to store the execution State")
			flags.Uint32Var(&mTrieCacheSize, "mtrie-cache-size", 1000, "cache size for MTrie")
			flags.UintVar(&checkpointDistance, "checkpoint-distance", 10, "number of WAL segments between checkpoints")
			flags.UintVar(&checkpointsToKeep, "checkpoints-to-keep", 5, "number of recent checkpoints to keep (0 to keep all)")
			flags.UintVar(&stateDeltasLimit, "state-deltas-limit", 1000, "maximum number of state deltas in the memory pool")
			flags.UintVar(&cadenceExecutionCache, "cadence-execution-cache", computation.DefaultProgramsCacheSize, "cache size for Cadence execution")
			flags.DurationVar(&requestInterval, "request-interval", 60*time.Second, "the interval between requests for the requester engine")
			flags.StringVar(&preferredExeNodeIDStr, "preferred-exe-node-id", "", "node ID for preferred execution node used for state sync")
			flags.UintVar(&transactionResultsCacheSize, "transaction-results-cache-size", 10000, "number of transaction results to be cached")
			flags.BoolVar(&syncByBlocks, "sync-by-blocks", true, "deprecated, sync by blocks instead of execution state deltas")
			flags.BoolVar(&syncFast, "sync-fast", false, "fast sync allows execution node to skip fetching collection during state syncing, and rely on state syncing to catch up")
			flags.IntVar(&syncThreshold, "sync-threshold", 100, "the maximum number of sealed and unexecuted blocks before triggering state syncing")
			flags.BoolVar(&extensiveLog, "extensive-logging", false, "extensive logging logs tx contents and block headers")
		}).
		Module("mutable follower state", func(node *cmd.FlowNodeBuilder) error {
			// For now, we only support state implementations from package badger.
			// If we ever support different implementations, the following can be replaced by a type-aware factory
			state, ok := node.State.(*badgerState.State)
			if !ok {
				return fmt.Errorf("only implementations of type badger.State are currenlty supported but read-only state has type %T", node.State)
			}
			followerState, err = badgerState.NewFollowerState(
				state,
				node.Storage.Index,
				node.Storage.Payloads,
				node.Tracer,
				node.ProtocolEvents,
			)
			return err
		}).
		Module("computation manager", func(node *cmd.FlowNodeBuilder) error {
			extraLogPath := path.Join(triedir, "extralogs")
			err := os.MkdirAll(extraLogPath, 0777)
			if err != nil {
				return fmt.Errorf("cannot create %s path for extrealogs: %w", extraLogPath, err)
			}

			extralog.ExtraLogDumpPath = extraLogPath

			rt := runtime.NewInterpreterRuntime()

			vm := fvm.New(rt)
			vmCtx := fvm.NewContext(node.Logger, node.FvmOptions...)

			manager, err := computation.New(
				node.Logger,
				collector,
				node.Tracer,
				node.Me,
				node.State,
				vm,
				vmCtx,
				cadenceExecutionCache,
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
			results = storage.NewExecutionResults(node.Metrics.Cache, node.DB)
			receipts = storage.NewExecutionReceipts(node.Metrics.Cache, node.DB, results)
			myReceipts = storage.NewMyExecutionReceipts(node.Metrics.Cache, node.DB, receipts)
			return nil
		}).
		Module("pending block cache", func(node *cmd.FlowNodeBuilder) error {
			pendingBlocks = buffer.NewPendingBlocks() // for following main chain consensus
			return nil
		}).
		Module("state deltas mempool", func(node *cmd.FlowNodeBuilder) error {
			deltas, err = ingestion.NewDeltas(stateDeltasLimit)
			return err
		}).
		Module("stake checking function", func(node *cmd.FlowNodeBuilder) error {
			checkStakedAtBlock = func(blockID flow.Identifier) (bool, error) {
				return protocol.IsNodeStakedAt(node.State.AtBlockID(blockID), node.Me.NodeID())
			}
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
					return nil, fmt.Errorf("mismatching root statecommitment. database has state commitment: %x, "+
						"bootstap has statecommitment: %x",
						commit, node.RootSeal.FinalState)
				}
			}

			ledgerStorage, err = ledger.NewLedger(triedir, int(mTrieCacheSize), collector, node.Logger.With().Str("subcomponent", "ledger").Logger(), node.MetricsRegisterer, ledger.DefaultPathFinderVersion)
			return ledgerStorage, err
		}).
		Component("execution state ledger WAL compactor", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			checkpointer, err := ledgerStorage.Checkpointer()
			if err != nil {
				return nil, fmt.Errorf("cannot create checkpointer: %w", err)
			}
			compactor := wal.NewCompactor(checkpointer, 10*time.Second, checkpointDistance, checkpointsToKeep)

			return compactor, nil
		}).
		Component("provider engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			chunkDataPacks := storage.NewChunkDataPacks(node.DB)
			stateCommitments := storage.NewCommits(node.Metrics.Cache, node.DB)

			// Needed for gRPC server, make sure to assign to main scoped vars
			events = storage.NewEvents(node.Metrics.Cache, node.DB)
			serviceEvents = storage.NewServiceEvents(node.Metrics.Cache, node.DB)
			txResults = storage.NewTransactionResults(node.Metrics.Cache, node.DB, transactionResultsCacheSize)

			executionState = state.NewExecutionState(
				ledgerStorage,
				stateCommitments,
				node.Storage.Blocks,
				node.Storage.Headers,
				node.Storage.Collections,
				chunkDataPacks,
				results,
				receipts,
				myReceipts,
				events,
				serviceEvents,
				txResults,
				node.DB,
				node.Tracer,
			)

			providerEngine, err = exeprovider.New(
				node.Logger,
				node.Tracer,
				node.Network,
				node.State,
				node.Me,
				executionState,
				collector,
				checkStakedAtBlock,
			)

			return providerEngine, err
		}).
		Component("checker engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			checkerEng = checker.New(
				node.Logger,
				node.State,
				executionState,
				node.Storage.Seals,
			)
			return checkerEng, nil
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

			ingestionEng, err = ingestion.New(
				node.Logger,
				node.Network,
				node.Me,
				collectionRequester,
				node.State,
				node.Storage.Blocks,
				node.Storage.Collections,
				events,
				serviceEvents,
				txResults,
				computationManager,
				providerEngine,
				executionState,
				collector,
				node.Tracer,
				extensiveLog,
				preferredExeFilter,
				deltas,
				syncThreshold,
				syncFast,
				checkStakedAtBlock,
			)

			// TODO: we should solve these mutual dependencies better
			// => https://github.com/dapperlabs/flow-go/issues/4360
			collectionRequester = collectionRequester.WithHandle(ingestionEng.OnCollection)

			node.ProtocolEvents.AddConsumer(ingestionEng)

			return ingestionEng, err
		}).
		Component("follower engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			// initialize cleaner for DB
			cleaner := storage.NewCleaner(node.Logger, node.DB, metrics.NewCleanerCollector(), flow.DefaultValueLogGCFrequency)

			// create a finalizer that handles updating the protocol
			// state when the follower detects newly finalized blocks
			final := finalizer.NewFinalizer(node.DB, node.Storage.Headers, followerState)

			// initialize the staking & beacon verifiers, signature joiner
			staking := signature.NewAggregationVerifier(encoding.ConsensusVoteTag)
			beacon := signature.NewThresholdVerifier(encoding.RandomBeaconTag)
			merger := signature.NewCombiner()

			// initialize consensus committee's membership state
			// This committee state is for the HotStuff follower, which follows the MAIN CONSENSUS Committee
			// Note: node.Me.NodeID() is not part of the consensus committee
			committee, err := committees.NewConsensusCommittee(node.State, node.Me.NodeID())
			if err != nil {
				return nil, fmt.Errorf("could not create Committee state for main consensus: %w", err)
			}

			// initialize the verifier for the protocol consensus
			verifier := verification.NewCombinedVerifier(committee, staking, beacon, merger)

			finalized, pending, err := recovery.FindLatest(node.State, node.Storage.Headers)
			if err != nil {
				return nil, fmt.Errorf("could not find latest finalized block and pending blocks to recover consensus follower: %w", err)
			}

			// creates a consensus follower with ingestEngine as the notifier
			// so that it gets notified upon each new finalized block
			followerCore, err := consensus.NewFollower(node.Logger, committee, node.Storage.Headers, final, verifier, checkerEng, node.RootBlock.Header, node.RootQC, finalized, pending)
			if err != nil {
				return nil, fmt.Errorf("could not create follower core logic: %w", err)
			}

			followerEng, err = followereng.New(
				node.Logger,
				node.Network,
				node.Me,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				cleaner,
				node.Storage.Headers,
				node.Storage.Payloads,
				followerState,
				pendingBlocks,
				followerCore,
				syncCore,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create follower engine: %w", err)
			}

			return followerEng, nil
		}).
		Component("collection requester engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			// We initialize the requester engine inside the ingestion engine due to the mutual dependency. However, in
			// order for it to properly start and shut down, we should still return it as its own engine here, so it can
			// be handled by the scaffold.
			return collectionRequester, nil
		}).
		Component("receipt provider engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			retrieve := func(blockID flow.Identifier) (flow.Entity, error) { return myReceipts.MyReceipt(blockID) }
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
		Component("sychronization engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			// initialize the synchronization engine
			syncEngine, err = synchronization.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.State,
				node.Storage.Blocks,
				followerEng,
				syncCore,
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
