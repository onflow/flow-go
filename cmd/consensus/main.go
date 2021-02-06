// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/blockproducer"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/requester"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/engine/consensus/compliance"
	"github.com/onflow/flow-go/engine/consensus/ingestion"
	"github.com/onflow/flow-go/engine/consensus/matching"
	"github.com/onflow/flow-go/engine/consensus/provider"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	builder "github.com/onflow/flow-go/module/builder/consensus"
	chmodule "github.com/onflow/flow-go/module/chunks"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/mempool"
	consensusMempools "github.com/onflow/flow-go/module/mempool/consensus"
	"github.com/onflow/flow-go/module/mempool/ejectors"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/module/validation"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/io"
)

func main() {

	var (
		guaranteeLimit                         uint
		resultLimit                            uint
		approvalLimit                          uint
		sealLimit                              uint
		pendngReceiptsLimit                    uint
		minInterval                            time.Duration
		maxInterval                            time.Duration
		maxSealPerBlock                        uint
		maxGuaranteePerBlock                   uint
		hotstuffTimeout                        time.Duration
		hotstuffMinTimeout                     time.Duration
		hotstuffTimeoutIncreaseFactor          float64
		hotstuffTimeoutDecreaseFactor          float64
		hotstuffTimeoutVoteAggregationFraction float64
		blockRateDelay                         time.Duration
		chunkAlpha                             uint
		requiredApprovalsForSealVerification   uint
		requiredApprovalsForSealConstruction   uint
		emergencySealing                       bool

		err              error
		mutableState     protocol.MutableState
		privateDKGData   *bootstrap.DKGParticipantPriv
		guarantees       mempool.Guarantees
		results          mempool.IncorporatedResults
		receipts         mempool.ExecutionTree
		approvals        mempool.Approvals
		seals            mempool.IncorporatedResultSeals
		pendingReceipts  mempool.PendingReceipts
		prov             *provider.Engine
		receiptRequester *requester.Engine
		syncCore         *synchronization.Core
		comp             *compliance.Engine
		conMetrics       module.ConsensusMetrics
		mainMetrics      module.HotstuffMetrics
		receiptValidator module.ReceiptValidator
		chunkAssigner    *chmodule.ChunkAssigner
	)

	cmd.FlowNode(flow.RoleConsensus.String()).
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.UintVar(&guaranteeLimit, "guarantee-limit", 1000, "maximum number of guarantees in the memory pool")
			flags.UintVar(&resultLimit, "result-limit", 10000, "maximum number of execution results in the memory pool")
			flags.UintVar(&approvalLimit, "approval-limit", 1000, "maximum number of result approvals in the memory pool")
			flags.UintVar(&sealLimit, "seal-limit", 10000, "maximum number of block seals in the memory pool")
			flags.UintVar(&pendngReceiptsLimit, "pending-receipts-limit", 10000, "maximum number of pending receipts in the mempool")
			flags.DurationVar(&minInterval, "min-interval", time.Millisecond, "the minimum amount of time between two blocks")
			flags.DurationVar(&maxInterval, "max-interval", 90*time.Second, "the maximum amount of time between two blocks")
			flags.UintVar(&maxSealPerBlock, "max-seal-per-block", 100, "the maximum number of seals to be included in a block")
			flags.UintVar(&maxGuaranteePerBlock, "max-guarantee-per-block", 100, "the maximum number of collection guarantees to be included in a block")
			flags.DurationVar(&hotstuffTimeout, "hotstuff-timeout", 60*time.Second, "the initial timeout for the hotstuff pacemaker")
			flags.DurationVar(&hotstuffMinTimeout, "hotstuff-min-timeout", 2500*time.Millisecond, "the lower timeout bound for the hotstuff pacemaker")
			flags.Float64Var(&hotstuffTimeoutIncreaseFactor, "hotstuff-timeout-increase-factor", timeout.DefaultConfig.TimeoutIncrease, "multiplicative increase of timeout value in case of time out event")
			flags.Float64Var(&hotstuffTimeoutDecreaseFactor, "hotstuff-timeout-decrease-factor", timeout.DefaultConfig.TimeoutDecrease, "multiplicative decrease of timeout value in case of progress")
			flags.Float64Var(&hotstuffTimeoutVoteAggregationFraction, "hotstuff-timeout-vote-aggregation-fraction", 0.6, "additional fraction of replica timeout that the primary will wait for votes")
			flags.DurationVar(&blockRateDelay, "block-rate-delay", 500*time.Millisecond, "the delay to broadcast block proposal in order to control block production rate")
			flags.UintVar(&chunkAlpha, "chunk-alpha", chmodule.DefaultChunkAssignmentAlpha, "number of verifiers that should be assigned to each chunk")
			flags.UintVar(&requiredApprovalsForSealVerification, "required-verification-seal-approvals", validation.DefaultRequiredApprovalsForSealValidation, "minimum number of approvals that are required to verify a seal")
			flags.UintVar(&requiredApprovalsForSealConstruction, "required-construction-seal-approvals", matching.DefaultRequiredApprovalsForSealConstruction, "minimum number of approvals that are required to construct a seal")
			flags.BoolVar(&emergencySealing, "emergency-sealing-active", matching.DefaultEmergencySealingActive, "(de)activation of emergency sealing")
		}).
		Module("consensus node metrics", func(node *cmd.FlowNodeBuilder) error {
			conMetrics = metrics.NewConsensusCollector(node.Tracer, node.MetricsRegisterer)
			return nil
		}).
		Module("mutable follower state", func(node *cmd.FlowNodeBuilder) error {
			// For now, we only support state implementations from package badger.
			// If we ever support different implementations, the following can be replaced by a type-aware factory
			state, ok := node.State.(*badgerState.State)
			if !ok {
				return fmt.Errorf("only implementations of type badger.State are currenlty supported but read-only state has type %T", node.State)
			}

			// We need to ensure `requiredApprovalsForSealVerification <= requiredApprovalsForSealConstruction <= chunkAlpha`
			if requiredApprovalsForSealVerification > requiredApprovalsForSealConstruction {
				return fmt.Errorf("invalid consensus parameters: requiredApprovalsForSealVerification > requiredApprovalsForSealConstruction")
			}
			if requiredApprovalsForSealConstruction > chunkAlpha {
				return fmt.Errorf("invalid consensus parameters: requiredApprovalsForSealConstruction > chunkAlpha")
			}

			chunkAssigner, err = chmodule.NewChunkAssigner(chunkAlpha, node.State)
			if err != nil {
				return fmt.Errorf("could not instantiate assignment algorithm for chunk verification: %w", err)
			}

			receiptValidator = validation.NewReceiptValidator(
				node.State,
				node.Storage.Index,
				node.Storage.Results,
				signature.NewAggregationVerifier(encoding.ExecutionReceiptTag))

			sealValidator := validation.NewSealValidator(
				node.State,
				node.Storage.Headers,
				node.Storage.Payloads,
				node.Storage.Seals,
				chunkAssigner,
				signature.NewAggregationVerifier(encoding.ResultApprovalTag),
				requiredApprovalsForSealVerification,
				conMetrics)

			mutableState, err = badgerState.NewFullConsensusState(
				state,
				node.Storage.Index,
				node.Storage.Payloads,
				node.Tracer,
				node.ProtocolEvents,
				receiptValidator,
				sealValidator)
			return err
		}).
		Module("random beacon key", func(node *cmd.FlowNodeBuilder) error {
			privateDKGData, err = loadDKGPrivateData(node.BaseConfig.BootstrapDir, node.NodeID)
			return err
		}).
		Module("collection guarantees mempool", func(node *cmd.FlowNodeBuilder) error {
			guarantees, err = stdmap.NewGuarantees(guaranteeLimit)
			return err
		}).
		Module("execution results mempool", func(node *cmd.FlowNodeBuilder) error {
			results, err = stdmap.NewIncorporatedResults(resultLimit)
			return err
		}).
		Module("execution receipts mempool", func(node *cmd.FlowNodeBuilder) error {
			receipts = consensusMempools.NewExecutionTree()
			// registers size method of backend for metrics
			err = node.Metrics.Mempool.Register(metrics.ResourceReceipt, receipts.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}
			return nil
		}).
		Module("result approvals mempool", func(node *cmd.FlowNodeBuilder) error {
			approvals, err = stdmap.NewApprovals(approvalLimit)
			return err
		}).
		Module("block seals mempool", func(node *cmd.FlowNodeBuilder) error {
			// use a custom ejector so we don't eject seals that would break
			// the chain of seals
			ejector := ejectors.NewLatestIncorporatedResultSeal(node.Storage.Headers)
			resultSeals := stdmap.NewIncorporatedResultSeals(stdmap.WithLimit(sealLimit), stdmap.WithEject(ejector.Eject))
			seals, err = consensusMempools.NewExecStateForkSuppressor(consensusMempools.LogForkAndCrash(node.Logger), resultSeals, node.DB, node.Logger)
			if err != nil {
				return fmt.Errorf("failed to wrap seals mempool into ExecStateForkSuppressor: %w", err)
			}
			return nil
		}).
		Module("pending receipts mempool", func(node *cmd.FlowNodeBuilder) error {
			pendingReceipts = stdmap.NewPendingReceipts(pendngReceiptsLimit)
			return nil
		}).
		Module("hotstuff main metrics", func(node *cmd.FlowNodeBuilder) error {
			mainMetrics = metrics.NewHotstuffCollector(node.RootChainID)
			return nil
		}).
		Module("sync core", func(node *cmd.FlowNodeBuilder) error {
			syncCore, err = synchronization.New(node.Logger, synchronization.DefaultConfig())
			return err
		}).
		Component("matching engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			receiptRequester, err = requester.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.State,
				engine.RequestReceiptsByBlockID,
				filter.HasRole(flow.RoleExecution),
				func() flow.Entity { return &flow.ExecutionReceipt{} },
				// requester.WithBatchThreshold(100),
			)
			if err != nil {
				return nil, err
			}

			match, err := matching.New(
				node.Logger,
				node.Metrics.Engine,
				node.Tracer,
				node.Metrics.Mempool,
				conMetrics,
				node.Network,
				node.State,
				node.Me,
				receiptRequester,
				node.Storage.Receipts,
				node.Storage.Headers,
				node.Storage.Index,
				results,
				receipts,
				approvals,
				seals,
				pendingReceipts,
				chunkAssigner,
				receiptValidator,
				requiredApprovalsForSealConstruction,
				emergencySealing,
			)

			receiptRequester.WithHandle(match.HandleReceipt)

			return match, err
		}).
		Component("provider engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			prov, err = provider.New(
				node.Logger,
				node.Metrics.Engine,
				node.Tracer,
				node.Network,
				node.State,
				node.Me,
			)
			return prov, err
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			ing, err := ingestion.New(
				node.Logger,
				node.Tracer,
				node.Metrics.Engine,
				conMetrics,
				node.Metrics.Mempool,
				node.Network,
				node.State,
				node.Storage.Headers,
				node.Me,
				guarantees,
			)
			return ing, err
		}).
		Component("consensus components", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			// TODO: we should probably find a way to initialize mutually dependent engines separately

			// initialize the entity database accessors
			cleaner := bstorage.NewCleaner(node.Logger, node.DB, metrics.NewCleanerCollector(), flow.DefaultValueLogGCFrequency)

			// initialize the pending blocks cache
			proposals := buffer.NewPendingBlocks()

			// initialize the compliance engine
			comp, err = compliance.New(
				node.Logger,
				node.Metrics.Engine,
				node.Tracer,
				node.Metrics.Mempool,
				conMetrics,
				node.Network,
				node.Me,
				cleaner,
				node.Storage.Headers,
				node.Storage.Payloads,
				mutableState,
				prov,
				proposals,
				syncCore,
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize compliance engine: %w", err)
			}

			// initialize the block builder
			var build module.Builder
			build = builder.NewBuilder(
				node.Metrics.Mempool,
				node.DB,
				mutableState,
				node.Storage.Headers,
				node.Storage.Seals,
				node.Storage.Index,
				node.Storage.Blocks,
				node.Storage.Results,
				guarantees,
				seals,
				receipts,
				node.Tracer,
				builder.WithMinInterval(minInterval),
				builder.WithMaxInterval(maxInterval),
				builder.WithMaxSealCount(maxSealPerBlock),
				builder.WithMaxGuaranteeCount(maxGuaranteePerBlock),
			)
			build = blockproducer.NewMetricsWrapper(build, mainMetrics) // wrapper for measuring time spent building block payload component

			// initialize the block finalizer
			finalize := finalizer.NewFinalizer(
				node.DB,
				node.Storage.Headers,
				mutableState,
				finalizer.WithCleanup(finalizer.CleanupMempools(
					node.Metrics.Mempool,
					conMetrics,
					node.Storage.Payloads,
					guarantees,
					seals,
				)),
			)

			// initialize the aggregating signature module for staking signatures
			staking := signature.NewAggregationProvider(encoding.ConsensusVoteTag, node.Me)

			// initialize the threshold signature module for random beacon signatures
			beacon := signature.NewThresholdProvider(encoding.RandomBeaconTag, privateDKGData.RandomBeaconPrivKey)

			// initialize the simple merger to combine staking & beacon signatures
			merger := signature.NewCombiner()

			// initialize Main consensus committee's state
			var committee hotstuff.Committee
			committee, err = committees.NewConsensusCommittee(node.State, node.Me.NodeID())
			if err != nil {
				return nil, fmt.Errorf("could not create Committee state for main consensus: %w", err)
			}
			committee = committees.NewMetricsWrapper(committee, mainMetrics) // wrapper for measuring time spent determining consensus committee relations

			// initialize the combined signer for hotstuff
			var signer hotstuff.SignerVerifier
			signer = verification.NewCombinedSigner(
				committee,
				staking,
				beacon,
				merger,
				node.NodeID,
			)
			signer = verification.NewMetricsWrapper(signer, mainMetrics) // wrapper for measuring time spent with crypto-related operations

			// initialize a logging notifier for hotstuff
			notifier := createNotifier(node.Logger, mainMetrics, node.Tracer, node.Storage.Index, node.RootChainID)
			// initialize the persister
			persist := persister.New(node.DB, node.RootChainID)

			// query the last finalized block and pending blocks for recovery
			finalized, pending, err := recovery.FindLatest(node.State, node.Storage.Headers)
			if err != nil {
				return nil, fmt.Errorf("could not find latest finalized block and pending blocks: %w", err)
			}

			// initialize hotstuff consensus algorithm
			hot, err := consensus.NewParticipant(
				node.Logger,
				notifier,
				mainMetrics,
				node.Storage.Headers,
				committee,
				build,
				finalize,
				persist,
				signer,
				comp,
				node.RootBlock.Header,
				node.RootQC,
				finalized,
				pending,
				consensus.WithInitialTimeout(hotstuffTimeout),
				consensus.WithMinTimeout(hotstuffMinTimeout),
				consensus.WithVoteAggregationTimeoutFraction(hotstuffTimeoutVoteAggregationFraction),
				consensus.WithTimeoutIncreaseFactor(hotstuffTimeoutIncreaseFactor),
				consensus.WithTimeoutDecreaseFactor(hotstuffTimeoutDecreaseFactor),
				consensus.WithBlockRateDelay(blockRateDelay),
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize hotstuff engine: %w", err)
			}

			comp = comp.WithConsensus(hot)
			return comp, nil
		}).
		Component("sync engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			sync, err := synceng.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.State,
				node.Storage.Blocks,
				comp,
				syncCore,
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize synchronization engine: %w", err)
			}

			return sync, nil
		}).
		Component("receipt requester engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			// created with matching engine
			return receiptRequester, nil
		}).
		Run()
}

func loadDKGPrivateData(dir string, myID flow.Identifier) (*bootstrap.DKGParticipantPriv, error) {
	path := fmt.Sprintf(bootstrap.PathRandomBeaconPriv, myID)
	data, err := io.ReadFile(filepath.Join(dir, path))
	if err != nil {
		return nil, err
	}

	var priv bootstrap.DKGParticipantPriv
	err = json.Unmarshal(data, &priv)
	if err != nil {
		return nil, err
	}
	return &priv, nil
}
