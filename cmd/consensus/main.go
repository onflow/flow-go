// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/onflow/flow-go/cmd/util/cmd/common"

	"github.com/spf13/pflag"

	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/blockproducer"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	recovery "github.com/onflow/flow-go/consensus/recovery/protocol"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/requester"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/engine/consensus/approvals/tracker"
	"github.com/onflow/flow-go/engine/consensus/compliance"
	dkgeng "github.com/onflow/flow-go/engine/consensus/dkg"
	"github.com/onflow/flow-go/engine/consensus/ingestion"
	"github.com/onflow/flow-go/engine/consensus/matching"
	"github.com/onflow/flow-go/engine/consensus/provider"
	"github.com/onflow/flow-go/engine/consensus/sealing"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/dkg"
	dkgmodel "github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/buffer"
	builder "github.com/onflow/flow-go/module/builder/consensus"
	chmodule "github.com/onflow/flow-go/module/chunks"
	dkgmodule "github.com/onflow/flow-go/module/dkg"
	"github.com/onflow/flow-go/module/epochs"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/mempool"
	consensusMempools "github.com/onflow/flow-go/module/mempool/consensus"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/module/validation"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/blocktimer"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/io"
)

func main() {

	var (
		guaranteeLimit                         uint
		resultLimit                            uint
		approvalLimit                          uint
		sealLimit                              uint
		pendingReceiptsLimit                   uint
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
		startupTimeString                      string
		startupTime                            time.Time

		// DKG contract client
		machineAccountInfo *bootstrap.NodeMachineAccountInfo
		flowClientOpts     []*common.FlowClientOpt
		insecureAccessAPI  bool
		accessNodeIDS      []string

		err                     error
		mutableState            protocol.MutableState
		privateDKGData          *dkgmodel.DKGParticipantPriv
		guarantees              mempool.Guarantees
		receipts                mempool.ExecutionTree
		seals                   mempool.IncorporatedResultSeals
		pendingReceipts         mempool.PendingReceipts
		prov                    *provider.Engine
		receiptRequester        *requester.Engine
		syncCore                *synchronization.Core
		comp                    *compliance.Engine
		conMetrics              module.ConsensusMetrics
		mainMetrics             module.HotstuffMetrics
		receiptValidator        module.ReceiptValidator
		chunkAssigner           *chmodule.ChunkAssigner
		finalizationDistributor *pubsub.FinalizationDistributor
		dkgBrokerTunnel         *dkgmodule.BrokerTunnel
		blockTimer              protocol.BlockTimer
		finalizedHeader         *synceng.FinalizedHeaderCache
	)

	nodeBuilder := cmd.FlowNode(flow.RoleConsensus.String())
	nodeBuilder.ExtraFlags(func(flags *pflag.FlagSet) {
		flags.UintVar(&guaranteeLimit, "guarantee-limit", 1000, "maximum number of guarantees in the memory pool")
		flags.UintVar(&resultLimit, "result-limit", 10000, "maximum number of execution results in the memory pool")
		flags.UintVar(&approvalLimit, "approval-limit", 1000, "maximum number of result approvals in the memory pool")
		// the default value is able to buffer as many seals as would be generated over ~12 hours. In case it
		// ever gets full, the node will simply crash instead of employing complex ejection logic.
		flags.UintVar(&sealLimit, "seal-limit", 44200, "maximum number of block seals in the memory pool")
		flags.UintVar(&pendingReceiptsLimit, "pending-receipts-limit", 10000, "maximum number of pending receipts in the mempool")
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
		flags.UintVar(&requiredApprovalsForSealConstruction, "required-construction-seal-approvals", sealing.DefaultRequiredApprovalsForSealConstruction, "minimum number of approvals that are required to construct a seal")
		flags.BoolVar(&emergencySealing, "emergency-sealing-active", sealing.DefaultEmergencySealingActive, "(de)activation of emergency sealing")
		flags.BoolVar(&insecureAccessAPI, "insecure-access-api", true, "required if insecure GRPC connection should be used")
		flags.StringArrayVar(&accessNodeIDS, "access-node-ids", []string{}, "array of access node ID's sorted in priority order where the first ID in this array will get the first connection attempt and each subsequent ID after serves as a fallback. minimum length 2")
		flags.StringVar(&startupTimeString, "hotstuff-startup-time", cmd.NotSet, "specifies date and time (in ISO 8601 format) after which the consensus participant may enter the first view (e.g 2006-01-02T15:04:05Z07:00)")
	})

	if err = nodeBuilder.Initialize(); err != nil {
		nodeBuilder.Logger.Fatal().Err(err).Send()
	}

	nodeBuilder.
		ValidateFlags(func() error {
			if startupTimeString != cmd.NotSet {
				t, err := time.Parse(time.RFC3339, startupTimeString)
				if err != nil {
					return fmt.Errorf("invalid start-time value: %w", err)
				}
				startupTime = t
			}
			return nil
		}).
		Module("consensus node metrics", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			conMetrics = metrics.NewConsensusCollector(node.Tracer, node.MetricsRegisterer)
			return nil
		}).
		Module("mutable follower state", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			// For now, we only support state implementations from package badger.
			// If we ever support different implementations, the following can be replaced by a type-aware factory
			state, ok := node.State.(*badgerState.State)
			if !ok {
				return fmt.Errorf("only implementations of type badger.State are currently supported but read-only state has type %T", node.State)
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
				node.Storage.Headers,
				node.Storage.Index,
				node.Storage.Results,
				node.Storage.Seals,
				signature.NewAggregationVerifier(encoding.ExecutionReceiptTag))

			resultApprovalSigVerifier := signature.NewAggregationVerifier(encoding.ResultApprovalTag)

			sealValidator, err := validation.NewSealValidator(
				node.State,
				node.Storage.Headers,
				node.Storage.Index,
				node.Storage.Results,
				node.Storage.Seals,
				chunkAssigner,
				resultApprovalSigVerifier,
				requiredApprovalsForSealConstruction,
				requiredApprovalsForSealVerification,
				conMetrics)
			if err != nil {
				return fmt.Errorf("could not instantiate seal validator: %w", err)
			}

			blockTimer, err = blocktimer.NewBlockTimer(minInterval, maxInterval)
			if err != nil {
				return err
			}

			mutableState, err = badgerState.NewFullConsensusState(
				state,
				node.Storage.Index,
				node.Storage.Payloads,
				node.Tracer,
				node.ProtocolEvents,
				blockTimer,
				receiptValidator,
				sealValidator)
			return err
		}).
		Module("random beacon key", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			// If this node was a participant in a spork, their DKG key for the
			// first epoch was generated during the bootstrapping process and is
			// specified in a private bootstrapping file. We load their key and
			// store it in the db for the initial post-spork epoch for use going
			// forward.
			// If this node was not a participant in a spork, they joined at an
			// epoch boundary, so they have no DKG file (they will generate
			// their first DKG private key through the procedure run during the
			// current epoch setup phase), and we do not need to insert a key at
			// startup.

			// if the node is not part of the current epoch identities, we do
			// not need to load the key
			epoch := node.State.AtBlockID(node.RootBlock.ID()).Epochs().Current()
			initialIdentities, err := epoch.InitialIdentities()
			if err != nil {
				return err
			}
			if _, ok := initialIdentities.ByNodeID(node.NodeID); !ok {
				node.Logger.Info().Msg("node joined at epoch boundary, not reading DKG file")
				return nil
			}

			// otherwise, load and save the key in DB for the current epoch (wrt
			// root block)
			privateDKGData, err = loadDKGPrivateData(node.BaseConfig.BootstrapDir, node.NodeID)
			if err != nil {
				return err
			}
			epochCounter, err := epoch.Counter()
			if err != nil {
				return err
			}
			err = node.Storage.DKGKeys.InsertMyDKGPrivateInfo(epochCounter, privateDKGData)
			if err != nil && !errors.Is(err, storage.ErrAlreadyExists) {
				return err
			}

			// Given an epoch, checkEpochKey returns an error if we are a
			// participant in the epoch and we don't have the corresponding DKG
			// key in the database.
			checkEpochKey := func(protocol.Epoch) error {
				identities, err := epoch.InitialIdentities()
				if err != nil {
					return err
				}
				if _, ok := identities.ByNodeID(node.NodeID); ok {
					counter, err := epoch.Counter()
					if err != nil {
						return err
					}
					_, err = node.Storage.DKGKeys.RetrieveMyDKGPrivateInfo(counter)
					if err != nil {
						return err
					}
				}
				return nil
			}

			// if we are a member of the current epoch, make sure we have the
			// DKG key
			currentEpoch := node.State.Final().Epochs().Current()
			err = checkEpochKey(currentEpoch)
			if err != nil {
				return fmt.Errorf("a random beacon that we are a participant in is currently in use and we don't have our key share for it: %w", err)
			}

			// if we participated in the DKG protocol for the next epoch, and we
			// are in EpochCommitted phase, make sure we have saved the
			// resulting DKG key
			phase, err := node.State.Final().Phase()
			if err != nil {
				return err
			}
			if phase == flow.EpochPhaseCommitted {
				nextEpoch := node.State.Final().Epochs().Next()
				err = checkEpochKey(nextEpoch)
				if err != nil {
					return fmt.Errorf("a random beacon DKG protocol that we were a participant in completed and we didn't store our key share for it: %w", err)
				}
			}

			return nil
		}).
		Module("collection guarantees mempool", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			guarantees, err = stdmap.NewGuarantees(guaranteeLimit)
			return err
		}).
		Module("execution receipts mempool", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			receipts = consensusMempools.NewExecutionTree()
			// registers size method of backend for metrics
			err = node.Metrics.Mempool.Register(metrics.ResourceReceipt, receipts.Size)
			if err != nil {
				return fmt.Errorf("could not register backend metric: %w", err)
			}
			return nil
		}).
		Module("block seals mempool", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			// use a custom ejector so we don't eject seals that would break
			// the chain of seals
			seals, err = consensusMempools.NewExecStateForkSuppressor(consensusMempools.LogForkAndCrash(node.Logger), node.DB, node.Logger, sealLimit)
			if err != nil {
				return fmt.Errorf("failed to wrap seals mempool into ExecStateForkSuppressor: %w", err)
			}
			err = node.Metrics.Mempool.Register(metrics.ResourcePendingIncorporatedSeal, seals.Size)
			return nil
		}).
		Module("pending receipts mempool", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			pendingReceipts = stdmap.NewPendingReceipts(node.Storage.Headers, pendingReceiptsLimit)
			return nil
		}).
		Module("hotstuff main metrics", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			mainMetrics = metrics.NewHotstuffCollector(node.RootChainID)
			return nil
		}).
		Module("sync core", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			syncCore, err = synchronization.New(node.Logger, synchronization.DefaultConfig())
			return err
		}).
		Module("finalization distributor", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			finalizationDistributor = pubsub.NewFinalizationDistributor()
			return nil
		}).
		Module("machine account config", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			machineAccountInfo, err = cmd.LoadNodeMachineAccountInfoFile(node.BootstrapDir, node.NodeID)
			return err
		}).
		Module("sdk client connection options", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) error {
			if len(accessNodeIDS) < common.DefaultAccessNodeIDSMinimum {
				return fmt.Errorf("invalid flag --access-node-ids atleast %x IDs must be provided", common.DefaultAccessNodeIDSMinimum)
			}

			flowClientOpts = make([]*common.FlowClientOpt, len(accessNodeIDS))
			for i, id := range accessNodeIDS {
				accessAddress, networkingPubKey, err := common.GetAccessNodeInfo(id, node.State.Sealed())
				if err != nil {
					return fmt.Errorf("failed to get networking info from protocol state for access node ID (%x): %s %w", i, id, err)
				}

				opt, err := common.NewFlowClientOpt(accessAddress, networkingPubKey, insecureAccessAPI)
				if err != nil {
					return fmt.Errorf("failed to get flow client connection option for access node ID (%x): %s %w", i, id, err)
				}

				flowClientOpts = append(flowClientOpts, opt)
			}

			return nil
		}).
		Component("machine account config validator", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			//@TODO use fallback logic for flowClient similar to DKG/QC contract clients
			flowClient, err := common.FlowClient(flowClientOpts[0])
			if err != nil {
				return nil, fmt.Errorf("failed to get flow client connection option for access node (0): %s %w", flowClientOpts[0].AccessAddress, err)
			}

			validator, err := epochs.NewMachineAccountConfigValidator(
				node.Logger,
				flowClient,
				flow.RoleCollection,
				*machineAccountInfo,
			)
			return validator, err
		}).
		Component("sealing engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

			resultApprovalSigVerifier := signature.NewAggregationVerifier(encoding.ResultApprovalTag)
			sealingTracker := tracker.NewSealingTracker(node.Logger, node.Storage.Headers, node.Storage.Receipts, seals)

			config := sealing.DefaultConfig()
			config.EmergencySealingActive = emergencySealing
			config.RequiredApprovalsForSealConstruction = requiredApprovalsForSealConstruction

			e, err := sealing.NewEngine(
				node.Logger,
				node.Tracer,
				conMetrics,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				sealingTracker,
				node.Network,
				node.Me,
				node.Storage.Headers,
				node.Storage.Payloads,
				node.Storage.Results,
				node.Storage.Index,
				node.State,
				node.Storage.Seals,
				chunkAssigner,
				resultApprovalSigVerifier,
				seals,
				config,
			)

			// subscribe for finalization events from hotstuff
			finalizationDistributor.AddOnBlockFinalizedConsumer(e.OnFinalizedBlock)
			finalizationDistributor.AddOnBlockIncorporatedConsumer(e.OnBlockIncorporated)

			return e, err
		}).
		Component("matching engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			receiptRequester, err = requester.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.State,
				engine.RequestReceiptsByBlockID,
				filter.HasRole(flow.RoleExecution),
				func() flow.Entity { return &flow.ExecutionReceipt{} },
				requester.WithRetryInitial(2*time.Second),
				requester.WithRetryMaximum(30*time.Second),
			)
			if err != nil {
				return nil, err
			}

			core := matching.NewCore(
				node.Logger,
				node.Tracer,
				conMetrics,
				node.Metrics.Mempool,
				node.State,
				node.Storage.Headers,
				node.Storage.Receipts,
				receipts,
				pendingReceipts,
				seals,
				receiptValidator,
				receiptRequester,
				matching.DefaultConfig(),
			)

			e, err := matching.NewEngine(
				node.Logger,
				node.Network,
				node.Me,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				node.State,
				node.Storage.Receipts,
				node.Storage.Index,
				core,
			)
			if err != nil {
				return nil, err
			}

			// subscribe engine to inputs from other node-internal components
			receiptRequester.WithHandle(e.HandleReceipt)
			finalizationDistributor.AddOnBlockFinalizedConsumer(e.OnFinalizedBlock)
			finalizationDistributor.AddOnBlockIncorporatedConsumer(e.OnBlockIncorporated)

			return e, err
		}).
		Component("provider engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
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
		Component("ingestion engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
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
		Component("consensus components", func(nodebuilder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

			// TODO: we should probably find a way to initialize mutually dependent engines separately

			// initialize the entity database accessors
			cleaner := bstorage.NewCleaner(node.Logger, node.DB, node.Metrics.CleanCollector, flow.DefaultValueLogGCFrequency)

			// initialize the pending blocks cache
			proposals := buffer.NewPendingBlocks()

			core, err := compliance.NewCore(node.Logger,
				node.Metrics.Engine,
				node.Tracer,
				node.Metrics.Mempool,
				node.Metrics.Compliance,
				cleaner,
				node.Storage.Headers,
				node.Storage.Payloads,
				mutableState,
				proposals,
				syncCore)
			if err != nil {
				return nil, fmt.Errorf("could not initialize compliance core: %w", err)
			}

			// initialize the compliance engine
			comp, err = compliance.NewEngine(node.Logger, node.Network, node.Me, prov, core)
			if err != nil {
				return nil, fmt.Errorf("could not initialize compliance engine: %w", err)
			}

			// initialize the block builder
			var build module.Builder
			build, err = builder.NewBuilder(
				node.Metrics.Mempool,
				node.DB,
				mutableState,
				node.Storage.Headers,
				node.Storage.Seals,
				node.Storage.Index,
				node.Storage.Blocks,
				node.Storage.Results,
				node.Storage.Receipts,
				guarantees,
				consensusMempools.NewIncorporatedResultSeals(seals, node.Storage.Receipts),
				receipts,
				node.Tracer,
				builder.WithBlockTimer(blockTimer),
				builder.WithMaxSealCount(maxSealPerBlock),
				builder.WithMaxGuaranteeCount(maxGuaranteePerBlock),
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialized block builder: %w", err)
			}

			build = blockproducer.NewMetricsWrapper(build, mainMetrics) // wrapper for measuring time spent building block payload component

			// initialize the block finalizer
			finalize := finalizer.NewFinalizer(
				node.DB,
				node.Storage.Headers,
				mutableState,
				node.Tracer,
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

			// initialize the verifier used to verify threshold signatures
			thresholdVerifier := signature.NewThresholdVerifier(encoding.RandomBeaconTag)

			// initialize the simple merger to combine staking & beacon signatures
			merger := signature.NewCombiner(encodable.ConsensusVoteSigLen, encodable.RandomBeaconSigLen)

			// initialize Main consensus committee's state
			var committee hotstuff.Committee
			committee, err = committees.NewConsensusCommittee(node.State, node.Me.NodeID())
			if err != nil {
				return nil, fmt.Errorf("could not create Committee state for main consensus: %w", err)
			}
			committee = committees.NewMetricsWrapper(committee, mainMetrics) // wrapper for measuring time spent determining consensus committee relations

			epochLookup := epochs.NewEpochLookup(node.State)

			thresholdSignerStore := signature.NewEpochAwareSignerStore(epochLookup, node.Storage.DKGKeys)

			// initialize the combined signer for hotstuff
			var signer hotstuff.SignerVerifier
			signer = verification.NewCombinedSigner(
				committee,
				staking,
				thresholdVerifier,
				merger,
				thresholdSignerStore,
				node.NodeID,
			)
			signer = verification.NewMetricsWrapper(signer, mainMetrics) // wrapper for measuring time spent with crypto-related operations

			// initialize a logging notifier for hotstuff
			notifier := createNotifier(
				node.Logger,
				mainMetrics,
				node.Tracer,
				node.Storage.Index,
				node.RootChainID,
			)

			notifier.AddConsumer(finalizationDistributor)

			// initialize the persister
			persist := persister.New(node.DB, node.RootChainID)

			// query the last finalized block and pending blocks for recovery
			finalized, pending, err := recovery.FindLatest(node.State, node.Storage.Headers)
			if err != nil {
				return nil, fmt.Errorf("could not find latest finalized block and pending blocks: %w", err)
			}

			opts := []consensus.Option{
				consensus.WithInitialTimeout(hotstuffTimeout),
				consensus.WithMinTimeout(hotstuffMinTimeout),
				consensus.WithVoteAggregationTimeoutFraction(hotstuffTimeoutVoteAggregationFraction),
				consensus.WithTimeoutIncreaseFactor(hotstuffTimeoutIncreaseFactor),
				consensus.WithTimeoutDecreaseFactor(hotstuffTimeoutDecreaseFactor),
				consensus.WithBlockRateDelay(blockRateDelay),
			}

			if !startupTime.IsZero() {
				opts = append(opts, consensus.WithStartupTime(startupTime))
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
				opts...,
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize hotstuff engine: %w", err)
			}

			comp = comp.WithConsensus(hot)
			return comp, nil
		}).
		Component("finalized snapshot", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			finalizedHeader, err = synceng.NewFinalizedHeaderCache(node.Logger, node.State, finalizationDistributor)
			if err != nil {
				return nil, fmt.Errorf("could not create finalized snapshot cache: %w", err)
			}

			return finalizedHeader, nil
		}).
		Component("sync engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			sync, err := synceng.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.Storage.Blocks,
				comp,
				syncCore,
				finalizedHeader,
				node.SyncEngineIdentifierProvider,
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize synchronization engine: %w", err)
			}

			return sync, nil
		}).
		Component("receipt requester engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			// created with sealing engine
			return receiptRequester, nil
		}).
		Component("DKG messaging engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {

			// brokerTunnel is used to forward messages between the DKG
			// messaging engine and the DKG broker/controller
			dkgBrokerTunnel = dkgmodule.NewBrokerTunnel()

			// messagingEngine is a network engine that is used by nodes to
			// exchange private DKG messages
			messagingEngine, err := dkgeng.NewMessagingEngine(
				node.Logger,
				node.Network,
				node.Me,
				dkgBrokerTunnel,
			)
			if err != nil {
				return nil, fmt.Errorf("could not initialize DKG messaging engine: %w", err)
			}

			return messagingEngine, nil
		}).
		Component("DKG reactor engine", func(builder cmd.NodeBuilder, node *cmd.NodeConfig) (module.ReadyDoneAware, error) {
			// the viewsObserver is used by the reactor engine to subscribe to
			// new views being finalized
			viewsObserver := gadgets.NewViews()
			node.ProtocolEvents.AddConsumer(viewsObserver)

			// keyDB is used to store the private key resulting from the node's
			// participation in the DKG run
			keyDB := badger.NewDKGKeys(node.Metrics.Cache, node.DB)

			// construct DKG contract client
			dkgContractClients, err := createDKGContractClients(node, machineAccountInfo, flowClientOpts)
			if err != nil {
				return nil, fmt.Errorf("could not create dkg contract client %w", err)
			}

			// the reactor engine reacts to new views being finalized and drives the
			// DKG protocol
			reactorEngine := dkgeng.NewReactorEngine(
				node.Logger,
				node.Me,
				node.State,
				keyDB,
				dkgmodule.NewControllerFactory(
					node.Logger,
					node.Me,
					dkgContractClients,
					dkgBrokerTunnel,
				),
				viewsObserver,
			)

			// reactorEngine consumes the EpochSetupPhaseStarted event
			node.ProtocolEvents.AddConsumer(reactorEngine)

			return reactorEngine, nil
		}).
		Run()
}

func loadDKGPrivateData(dir string, myID flow.Identifier) (*dkg.DKGParticipantPriv, error) {
	path := fmt.Sprintf(bootstrap.PathRandomBeaconPriv, myID)
	data, err := io.ReadFile(filepath.Join(dir, path))
	if err != nil {
		return nil, err
	}

	var priv dkg.DKGParticipantPriv
	err = json.Unmarshal(data, &priv)
	if err != nil {
		return nil, err
	}
	return &priv, nil
}

// createDKGContractClient creates an dkgContractClient
func createDKGContractClient(node *cmd.NodeConfig, machineAccountInfo *bootstrap.NodeMachineAccountInfo, flowClient *client.Client) (module.DKGContractClient, error) {
	var dkgClient module.DKGContractClient

	contracts, err := systemcontracts.SystemContractsForChain(node.RootChainID)
	if err != nil {
		return nil, err
	}
	dkgContractAddress := contracts.DKG.Address.Hex()

	// construct signer from private key
	sk, err := crypto.DecodePrivateKey(machineAccountInfo.SigningAlgorithm, machineAccountInfo.EncodedPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("could not decode private key from hex: %w", err)
	}
	txSigner := crypto.NewInMemorySigner(sk, machineAccountInfo.HashAlgorithm)

	// create actual dkg contract client, all flags and machine account info file found
	dkgClient = dkgmodule.NewClient(
		node.Logger,
		flowClient,
		txSigner,
		dkgContractAddress,
		machineAccountInfo.Address,
		machineAccountInfo.KeyIndex,
	)

	return dkgClient, nil
}

// createDKGContractClients creates an array dkgContractClient that is sorted by retry fallback priority
func createDKGContractClients(node *cmd.NodeConfig, machineAccountInfo *bootstrap.NodeMachineAccountInfo, flowClientOpts []*common.FlowClientOpt) ([]module.DKGContractClient, error) {
	dkgClients := make([]module.DKGContractClient, len(flowClientOpts))

	for _, opt := range flowClientOpts {
		flowClient, err := common.FlowClient(opt)
		if err != nil {
			return nil, fmt.Errorf("failed to create flow client for dkg contract client with options: %s %w", flowClientOpts, err)
		}

		dkgClient, err := createDKGContractClient(node, machineAccountInfo, flowClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create dkg contract client with flow client options: %s %w", flowClientOpts, err)
		}

		dkgClients = append(dkgClients, dkgClient)
	}

	return dkgClients, nil
}
