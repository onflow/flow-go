package consensus

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/blockproducer"
	"github.com/onflow/flow-go/consensus/hotstuff/eventhandler"
	"github.com/onflow/flow-go/consensus/hotstuff/eventloop"
	"github.com/onflow/flow-go/consensus/hotstuff/forks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/onflow/flow-go/consensus/hotstuff/safetyrules"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	validatorImpl "github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/consensus/recovery"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

// NewParticipant initialize the EventLoop instance with needed dependencies
func NewParticipant(
	log zerolog.Logger,
	metrics module.HotstuffMetrics,
	mempoolMetrics module.MempoolMetrics,
	builder module.Builder,
	finalized *flow.Header,
	pending []*flow.Header,
	modules *HotstuffModules,
	options ...Option,
) (*eventloop.EventLoop, error) {

	// initialize the default configuration and apply the configuration options
	cfg := DefaultParticipantConfig()
	for _, option := range options {
		option(&cfg)
	}

	// prune vote aggregator to initial view
	modules.VoteAggregator.PruneUpToView(finalized.View)
	modules.TimeoutAggregator.PruneUpToView(finalized.View)

	// recover HotStuff state from all pending blocks
	qcCollector := recovery.NewCollector[*flow.QuorumCertificate]()
	tcCollector := recovery.NewCollector[*flow.TimeoutCertificate]()
	err := recovery.Recover(log, pending,
		recovery.ForksState(modules.Forks),                   // add pending blocks to Forks
		recovery.VoteAggregatorState(modules.VoteAggregator), // accept votes for all pending blocks
		recovery.CollectParentQCs(qcCollector),               // collect QCs from all pending block to initialize PaceMaker (below)
		recovery.CollectTCs(tcCollector),                     // collect TCs from all pending block to initialize PaceMaker (below)
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan tree of pending blocks: %w", err)
	}

	// initialize dynamically updatable timeout config
	timeoutConfig, err := timeout.NewConfig(cfg.TimeoutMinimum, cfg.TimeoutMaximum, cfg.TimeoutAdjustmentFactor, cfg.HappyPathMaxRoundFailures, cfg.MaxTimeoutObjectRebroadcastInterval)
	if err != nil {
		return nil, fmt.Errorf("could not initialize timeout config: %w", err)
	}

	// initialize the pacemaker
	controller := timeout.NewController(timeoutConfig)
	pacemaker, err := pacemaker.New(controller, cfg.ProposalDurationProvider, modules.Notifier, modules.Persist,
		pacemaker.WithQCs(qcCollector.Retrieve()...),
		pacemaker.WithTCs(tcCollector.Retrieve()...),
	)
	if err != nil {
		return nil, fmt.Errorf("could not initialize flow pacemaker: %w", err)
	}

	// initialize block producer
	producer, err := blockproducer.New(modules.Signer, modules.Committee, builder)
	if err != nil {
		return nil, fmt.Errorf("could not initialize block producer: %w", err)
	}

	// initialize the safetyRules
	safetyRules, err := safetyrules.New(modules.Signer, modules.Persist, modules.Committee)
	if err != nil {
		return nil, fmt.Errorf("could not initialize safety rules: %w", err)
	}

	// initialize the event handler
	eventHandler, err := eventhandler.NewEventHandler(
		log,
		pacemaker,
		producer,
		modules.Forks,
		modules.Persist,
		modules.Committee,
		safetyRules,
		modules.Notifier,
	)
	if err != nil {
		return nil, fmt.Errorf("could not initialize event handler: %w", err)
	}

	// initialize and return the event loop
	loop, err := eventloop.NewEventLoop(log, metrics, mempoolMetrics, eventHandler, cfg.StartupTime)
	if err != nil {
		return nil, fmt.Errorf("could not initialize event loop: %w", err)
	}

	// add observer, event loop needs to receive events from distributor
	modules.VoteCollectorDistributor.AddVoteCollectorConsumer(loop)
	modules.TimeoutCollectorDistributor.AddTimeoutCollectorConsumer(loop)

	return loop, nil
}

// NewValidator creates new instance of hotstuff validator needed for votes & proposal validation
func NewValidator(metrics module.HotstuffMetrics, committee hotstuff.DynamicCommittee) hotstuff.Validator {
	packer := signature.NewConsensusSigDataPacker(committee)
	verifier := verification.NewCombinedVerifier(committee, packer)

	// initialize the Validator
	validator := validatorImpl.New(committee, verifier)
	return validatorImpl.NewMetricsWrapper(validator, metrics) // wrapper for measuring time spent in Validator component
}

// NewForks recovers trusted root and creates new forks manager
func NewForks(final *flow.Header, headers storage.Headers, updater module.Finalizer, notifier hotstuff.FollowerConsumer, rootHeader *flow.Header, rootQC *flow.QuorumCertificate) (*forks.Forks, error) {
	// recover the trusted root
	trustedRoot, err := recoverTrustedRoot(final, headers, rootHeader, rootQC)
	if err != nil {
		return nil, fmt.Errorf("could not recover trusted root: %w", err)
	}

	// initialize the forks
	forks, err := forks.New(trustedRoot, updater, notifier)
	if err != nil {
		return nil, fmt.Errorf("could not initialize forks: %w", err)
	}

	return forks, nil
}

// recoverTrustedRoot based on our local state returns root block and QC that can be used to initialize base state
func recoverTrustedRoot(final *flow.Header, headers storage.Headers, rootHeader *flow.Header, rootQC *flow.QuorumCertificate) (*model.CertifiedBlock, error) {
	if final.View < rootHeader.View {
		return nil, fmt.Errorf("finalized Block has older view than trusted root")
	}

	// if finalized view is genesis block, then use genesis block as the trustedRoot
	if final.View == rootHeader.View {
		if final.ID() != rootHeader.ID() {
			return nil, fmt.Errorf("finalized Block conflicts with trusted root")
		}
		certifiedRoot, err := makeCertifiedRootBlock(rootHeader, rootQC)
		if err != nil {
			return nil, fmt.Errorf("constructing certified root block failed: %w", err)
		}
		return &certifiedRoot, nil
	}

	// find a valid child of the finalized block in order to get its QC
	children, err := headers.ByParentID(final.ID())
	if err != nil {
		// a finalized block must have a valid child, if err happens, we exit
		return nil, fmt.Errorf("could not get children for finalized block (ID: %v, view: %v): %w", final.ID(), final.View, err)
	}
	if len(children) == 0 {
		return nil, fmt.Errorf("finalized block has no children")
	}

	child := model.BlockFromFlow(children[0])

	// create the root block to use
	trustedRoot, err := model.NewCertifiedBlock(model.BlockFromFlow(final), child.QC)
	if err != nil {
		return nil, fmt.Errorf("constructing certified root block failed: %w", err)
	}
	return &trustedRoot, nil
}

func makeCertifiedRootBlock(header *flow.Header, qc *flow.QuorumCertificate) (model.CertifiedBlock, error) {
	// By convention of Forks, the trusted root block does not need to have a qc
	// (as is the case for the genesis block). For simplify of the implementation, we always omit
	// the QC of the root block. Thereby, we have one algorithm which handles all cases,
	// instead of having to distinguish between a genesis block without a qc
	// and a later-finalized root block where we can retrieve the qc.
	rootBlock := &model.Block{
		View:        header.View,
		BlockID:     header.ID(),
		ProposerID:  header.ProposerID,
		QC:          nil, // QC is omitted
		PayloadHash: header.PayloadHash,
		Timestamp:   header.Timestamp,
	}
	return model.NewCertifiedBlock(rootBlock, qc)
}
