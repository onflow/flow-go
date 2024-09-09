package blockproducer

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// safetyRulesConcurrencyWrapper wraps `hotstuff.SafetyRules` to allow its use in concurrent environment.
// Correctness requirements:
//
//	 (i) The wrapper's Sign function is called exactly once (wrapper errors on repeated Sign calls)
//	(ii) SafetyRules is not accessed outside the wrapper concurrently. The wrapper cannot enforce this.
//
// The correctness condition (ii) holds because there is a single dedicated thread executing the Event Loop,
// including the EventHandler, that also runs the logic of `BlockProducer.MakeBlockProposal`.
//
// Concurrency safety:
//
//	(a) There is one dedicated thread executing the Event Loop, including the EventHandler, that also runs the logic of
//	    `BlockProducer.MakeBlockProposal`. Hence, while the "Event Loop Thread" is in `MakeBlockProposal`, we are guaranteed
//	    the only interactions with `SafetyRules` are in `module.Builder.BuildOn`
//	(b) The Event Loop Thread instantiates the variable `signingStatus`. Furthermore, the `signer` call first reads `signingStatus`.
//	    Therefore, all operations in the EventHandler prior to calling `Builder.BuildOn(..)` happen before the call to `signer`.
//	    Hence, it is guaranteed that the `signer` uses the most recent state of `SafetyRules`, even if signer is executed by a
//	    different thread.
//	(c) Just before the `signer` call returns, it writes `signingStatus`. Furthermore, the Event Loop Thread reads `signingStatus`
//	    right after the `Builder.BuildOn(..)` call returns. Thereby, Event Loop Thread sees the most recent state of `SafetyRules`
//	    after completing the signing operation
//
// With the transitivity of the 'Happens Before' relationship (-> go Memory Model https://go.dev/ref/mem#atomic), we have proven
// that concurrent access of the wrapped `safetyRules` is safe for the state transition:
//
//	instantiate signingStatus to 0  ─►  update signingStatus from 0 to 1 → signer → update signingStatus from 1 to 2  ─►  confirm signingStatus has value 2
//
// ╰──────────────┬───────────────╯    ╰──────────────────────────────────────┬─────────────────────────────────────╯    ╰────────────────┬────────────────╯
//
//	Event Loop Thread                                 within the scope of Builder.BuildOn                                  Event Loop Thread
//
// All state transitions _other_ than the one above yield exceptions without modifying `SafetyRules`.
type safetyRulesConcurrencyWrapper struct {
	// signingStatus guarantees concurrency safety and encodes the progress of the signing process.
	// We differentiate between 4 different states:
	//  - value 0: signing is not yet started
	//  - value 1: one thread has already entered the signing process, which is currently ongoing
	//  - value 2: the thread that set `signingStatus` to value 1 has completed the signing
	//  - value 3: proposal has been produced and signed and Event Loop thread has continued executing the `BlockProducer` logic
	signingStatus atomic.Uint32
	safetyRules   hotstuff.SafetyRules
}

func newSafetyRulesConcurrencyWrapper(safetyRules hotstuff.SafetyRules) *safetyRulesConcurrencyWrapper {
	return &safetyRulesConcurrencyWrapper{safetyRules: safetyRules}
}

// Sign modifies the given unsignedHeader by including the proposer's signature date.
// Safe under concurrent calls. Per convention, this method should be called exactly once.
// Only the first call will succeed, and subsequent calls error. The implementation is backed
// by `SafetyRules` and thereby guarantees consensus safety for singing block proposals.
// No errors expected during normal operations
func (w *safetyRulesConcurrencyWrapper) Sign(unsignedHeader *flow.Header) error {
	if !w.signingStatus.CompareAndSwap(0, 1) { // value of `signingStatus` is something else than 0
		return fmt.Errorf("signer has already commenced signing; possebly repeated signer call")
	} // signer is now in state 1, and this thread is the only one every going to execute the following logic

	// signature for own block is structurally a vote
	vote, err := w.safetyRules.SignOwnProposal(model.ProposalFromFlow(unsignedHeader))
	if err != nil {
		return fmt.Errorf("could not sign block proposal: %w", err)
	}
	unsignedHeader.ProposerSigData = vote.SigData

	// value of `signingStatus` is always 1, i.e. the following check always succeeds.
	if !w.signingStatus.CompareAndSwap(1, 2) { // sanity check protects logic from future modifications accidentally breaking this invariant
		panic("signer wrapper completed its work but encountered state other than 1") // never happens
	}
	return nil
}

// IsSigningComplete atomically checks whether the Sign logic has concluded, and returns true exaclty in this case.
// By reading the atomic `signingStatus` and confirming it has the expected value, it is guaranteed that any state
// changes of `safetyRules` that happened within `Sign` are visible to the Event Loop Thread.
// No errors expected during normal operations
func (w *safetyRulesConcurrencyWrapper) IsSigningComplete() bool {
	return w.signingStatus.Load() == 2
}
