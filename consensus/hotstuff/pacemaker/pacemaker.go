package pacemaker

import (
	"context"
	"fmt"
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/onflow/flow-go/model/flow"
)

// ActivePaceMaker implements the hotstuff.PaceMaker
// Conceptually, we use the Pacemaker algorithm first proposed in [1] (specifically Jolteon) and described in more detail in [2].
// [1] https://arxiv.org/abs/2106.10362
// [2] https://developers.diem.com/papers/diem-consensus-state-machine-replication-in-the-diem-blockchain/2021-08-17.pdf (aka DiemBFT v4)
//
// To enter a new view `v`, the Pacemaker must observe a valid QC or TC for view `v-1`.
// The Pacemaker also controls when a node should locally time out for a given view.
// In contrast to the passive Pacemaker (previous implementation), locally timing a view
// does not cause a view change.
// A local timeout for a view `v` causes a node to:
// * never produce a vote for any proposal with view ≤ `v`, after the timeout
// * produce and broadcast a timeout object, which can form a part of the TC for the timed out view
//
// Not concurrency safe.
type ActivePaceMaker struct {
	ctx            context.Context
	timeoutControl *timeout.Controller
	notifier       hotstuff.Consumer
	viewTracker    viewTracker
	started        bool
}

var _ hotstuff.PaceMaker = (*ActivePaceMaker)(nil)

// New creates a new ActivePaceMaker instance
//   - startView is the view for the pacemaker to start with.
//   - timeoutController controls the timeout trigger.
//   - notifier provides callbacks for pacemaker events.
//
// Expected error conditions:
// * model.ConfigurationError if initial LivenessData is invalid
func New(
	timeoutController *timeout.Controller,
	notifier hotstuff.Consumer,
	persist hotstuff.Persister,
	recovery ...recoveryInformation,
) (*ActivePaceMaker, error) {
	vt, err := newViewTracker(persist)
	if err != nil {
		return nil, fmt.Errorf("initializing view tracker failed: %w", err)
	}

	pm := &ActivePaceMaker{
		timeoutControl: timeoutController,
		notifier:       notifier,
		viewTracker:    vt,
		started:        false,
	}
	for _, recoveryAction := range recovery {
		err = recoveryAction(pm)
		if err != nil {
			return nil, fmt.Errorf("ingesting recovery information failed: %w", err)
		}
	}
	return pm, nil
}

// CurView returns the current view
func (p *ActivePaceMaker) CurView() uint64 { return p.viewTracker.CurView() }

// NewestQC returns QC with the highest view discovered by PaceMaker.
func (p *ActivePaceMaker) NewestQC() *flow.QuorumCertificate { return p.viewTracker.NewestQC() }

// LastViewTC returns TC for last view, this will be nil only if the current view
// was entered with a QC.
func (p *ActivePaceMaker) LastViewTC() *flow.TimeoutCertificate { return p.viewTracker.LastViewTC() }

// TimeoutChannel returns the timeout channel for current active timeout.
// Note the returned timeout channel returns only one timeout, which is the current
// timeout.
// To get the timeout for the next timeout, you need to call TimeoutChannel() again.
func (p *ActivePaceMaker) TimeoutChannel() <-chan time.Time { return p.timeoutControl.Channel() }

// BlockRateDelay returns the delay for broadcasting its own proposals.
func (p *ActivePaceMaker) BlockRateDelay() time.Duration { return p.timeoutControl.BlockRateDelay() }

// ProcessQC notifies the pacemaker with a new QC, which might allow pacemaker to
// fast-forward its view. In contrast to `ProcessTC`, this function does _not_ handle `nil` inputs.
// No errors are expected, any error should be treated as exception
func (p *ActivePaceMaker) ProcessQC(qc *flow.QuorumCertificate) (*model.NewViewEvent, error) {
	initialView := p.CurView()
	resultingView, err := p.viewTracker.ProcessQC(qc)
	if err != nil {
		return nil, fmt.Errorf("unexpected exception in viewTracker while processing QC for view %d: %w", qc.View, err)
	}
	if resultingView <= initialView {
		return nil, nil
	}

	// TC triggered view change:
	p.timeoutControl.OnProgressBeforeTimeout()
	p.notifier.OnQcTriggeredViewChange(initialView, resultingView, qc)

	p.notifier.OnViewChange(initialView, resultingView)
	timerInfo := p.timeoutControl.StartTimeout(p.ctx, resultingView)
	p.notifier.OnStartingTimeout(timerInfo)

	return &model.NewViewEvent{
		View:      timerInfo.View,
		StartTime: timerInfo.StartTime,
		Duration:  timerInfo.Duration,
	}, nil
}

// ProcessTC notifies the Pacemaker of a new timeout certificate, which may allow
// Pacemaker to fast-forward its current view.
// A nil TC is an expected valid input, so that callers may pass in e.g. `Proposal.LastViewTC`,
// which may or may not have a value.
// No errors are expected, any error should be treated as exception
func (p *ActivePaceMaker) ProcessTC(tc *flow.TimeoutCertificate) (*model.NewViewEvent, error) {
	initialView := p.CurView()
	resultingView, err := p.viewTracker.ProcessTC(tc)
	if err != nil {
		return nil, fmt.Errorf("unexpected exception in viewTracker while processing TC for view %d: %w", tc.View, err)
	}
	if resultingView <= initialView {
		return nil, nil
	}

	// TC triggered view change:
	p.timeoutControl.OnTimeout()
	p.notifier.OnTcTriggeredViewChange(initialView, resultingView, tc)

	p.notifier.OnViewChange(initialView, resultingView)
	timerInfo := p.timeoutControl.StartTimeout(p.ctx, resultingView)
	p.notifier.OnStartingTimeout(timerInfo)

	return &model.NewViewEvent{
		View:      timerInfo.View,
		StartTime: timerInfo.StartTime,
		Duration:  timerInfo.Duration,
	}, nil
}

// Start starts the pacemaker by starting the initial timer for the current view.
// Start should only be called once - subsequent calls are a no-op.
// CAUTION: ActivePaceMaker is not concurrency safe. The Start method must
// be executed by the same goroutine that also calls the other business logic
// methods, or concurrency safety has to be implemented externally.
func (p *ActivePaceMaker) Start(ctx context.Context) {
	if p.started {
		return
	}
	p.started = true
	p.ctx = ctx
	timerInfo := p.timeoutControl.StartTimeout(ctx, p.CurView())
	p.notifier.OnStartingTimeout(timerInfo)
}

/* ------------------------------------ recovery parameters for PaceMaker ------------------------------------ */

// recoveryInformation provides optional information to the PaceMaker during its construction
// to ingest additional information that was potentially lost during a crash or reboot.
// Following the "information-driven" approach, we consider potentially older or redundant
// information as consistent with our already-present knowledge, i.e. as a no-op.
type recoveryInformation func(p *ActivePaceMaker) error

// WithQC informs the PaceMaker about the given QC. Old and nil QCs are accepted (no-op).
func WithQC(qc *flow.QuorumCertificate) recoveryInformation {
	// For recovery, we allow the special case of a nil QC, because the genesis block has no QC.
	if qc == nil {
		return func(p *ActivePaceMaker) error { return nil } // no-op
	}
	return func(p *ActivePaceMaker) error {
		_, err := p.viewTracker.ProcessQC(qc)
		return err
	}
}

// WithTC informs the PaceMaker about the given TC. Old and nil TCs are accepted (no-op).
func WithTC(tc *flow.TimeoutCertificate) recoveryInformation {
	// Business logic accepts nil TC already, as this is the common case on the happy path.
	return func(p *ActivePaceMaker) error {
		_, err := p.viewTracker.ProcessTC(tc)
		return err
	}
}
