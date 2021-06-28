// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"
	"google.golang.org/grpc"

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
	"github.com/onflow/flow-go/engine/consensus/approvals"
	"github.com/onflow/flow-go/engine/consensus/compliance"
	dkgeng "github.com/onflow/flow-go/engine/consensus/dkg"
	"github.com/onflow/flow-go/engine/consensus/ingestion"
	"github.com/onflow/flow-go/engine/consensus/matching"
	"github.com/onflow/flow-go/engine/consensus/provider"
	"github.com/onflow/flow-go/engine/consensus/sealing"
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
		accessAddress                          string

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
			flags.UintVar(&requiredApprovalsForSealConstruction, "required-construction-seal-approvals", sealing.DefaultRequiredApprovalsForSealConstruction, "minimum number of approvals that are required to construct a seal")
			flags.BoolVar(&emergencySealing, "emergency-sealing-active", sealing.DefaultEmergencySealingActive, "(de)activation of emergency sealing")
			flags.StringVar(&accessAddress, "access-address", "", "the address of an access node")
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
				node.Storage.Headers,
				node.Storage.Index,
				node.Storage.Results,
				node.Storage.Seals,
				signature.NewAggregationVerifier(encoding.ExecutionReceiptTag))

			resultApprovalSigVerifier := signature.NewAggregationVerifier(encoding.ResultApprovalTag)

			sealValidator := validation.NewSealValidator(
				node.State,
				node.Storage.Headers,
				node.Storage.Index,
				node.Storage.Results,
				node.Storage.Seals,
				chunkAssigner,
				resultApprovalSigVerifier,
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
		Module("collection guarantees mempool", func(node *cmd.FlowNodeBuilder) error {
			guarantees, err = stdmap.NewGuarantees(guaranteeLimit)
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
		Module("block seals mempool", func(node *cmd.FlowNodeBuilder) error {
			resultSeals := stdmap.NewIncorporatedResultSeals(sealLimit)
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
		Module("finalization distributor", func(node *cmd.FlowNodeBuilder) error {
			finalizationDistributor = pubsub.NewFinalizationDistributor()
			return nil
		}).
		Component("sealing engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			resultApprovalSigVerifier := signature.NewAggregationVerifier(encoding.ResultApprovalTag)

			config := sealing.DefaultConfig()
			config.EmergencySealingActive = emergencySealing
			config.RequiredApprovalsForSealConstruction = requiredApprovalsForSealConstruction

			e, err := sealing.NewEngine(
				node.Logger,
				node.Tracer,
				conMetrics,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				node.Network,
				node.Me,
				node.Storage.Headers,
				node.Storage.Payloads,
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
				core,
			)
			if err != nil {
				return nil, err
			}

			// subscribe engine to inputs from other node-internal components
			receiptRequester.WithHandle(e.HandleReceipt)
			finalizationDistributor.AddOnBlockFinalizedConsumer(e.OnFinalizedBlock)

			return e, err
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
				return nil, fmt.Errorf("coult not initialize compliance core: %w", err)
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
				approvals.NewIncorporatedResultSeals(seals, node.Storage.Receipts),
				receipts,
				node.Tracer,
				builder.WithMinInterval(minInterval),
				builder.WithMaxInterval(maxInterval),
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
			// created with sealing engine
			return receiptRequester, nil
		}).
		Component("DKG messaging engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

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
		Component("DKG reactor engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			// the viewsObserver is used by the reactor engine to subscribe to
			// new views being finalized
			viewsObserver := gadgets.NewViews()
			node.ProtocolEvents.AddConsumer(viewsObserver)

			// keyDB is used to store the private key resulting from the node's
			// participation in the DKG run
			keyDB := badger.NewDKGKeys(node.Metrics.Cache, node.DB)

			machineAccountInfo, err := loadMachineAccountPrivateData(node)
			if err != nil {
				return nil, fmt.Errorf("could't load machine account info: %w", err)
			}
			decodedPrivateKey, err := crypto.DecodePrivateKey(
				machineAccountInfo.SigningAlgorithm,
				machineAccountInfo.EncodedPrivateKey,
			)
			if err != nil {
				return nil, fmt.Errorf("could not decode machine account private key: %w", err)
			}
			machineAccountSigner := crypto.NewInMemorySigner(
				decodedPrivateKey,
				machineAccountInfo.HashAlgorithm,
			)

			sdkClient, err := client.New(
				accessAddress,
				grpc.WithInsecure())
			if err != nil {
				return nil, fmt.Errorf("could not initialise sdk client: %w", err)
			}

			dkgContractClient := dkgmodule.NewClient(
				node.Logger,
				sdkClient,
				machineAccountSigner,
				node.RootChainID.Chain().ServiceAddress().HexWithPrefix(),
				machineAccountInfo.Address,
				machineAccountInfo.KeyIndex,
			)

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
					dkgContractClient,
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

func loadMachineAccountPrivateData(node *cmd.FlowNodeBuilder) (*bootstrap.NodeMachineAccountInfo, error) {
	data, err := io.ReadFile(filepath.Join(node.BaseConfig.BootstrapDir, fmt.Sprintf(bootstrap.PathNodeMachineAccountInfoPriv, node.Me.NodeID())))
	if err != nil {
		return nil, err
	}
	var info bootstrap.NodeMachineAccountInfo
	err = json.Unmarshal(data, &info)
	return &info, err
}
