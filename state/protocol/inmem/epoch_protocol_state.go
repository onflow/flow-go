package inmem

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// EpochProtocolStateAdapter implements protocol.EpochProtocolState by wrapping a flow.RichEpochStateEntry.
//
//structwrite:immutable - mutations allowed only within the constructor
type EpochProtocolStateAdapter struct {
	*flow.RichEpochStateEntry
	Params protocol.GlobalParams
}

var _ protocol.EpochProtocolState = (*EpochProtocolStateAdapter)(nil)

// UntrustedEpochProtocolStateAdapter is an untrusted input-only representation of a EpochProtocolStateAdapter,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedEpochProtocolStateAdapter should be validated and converted into
// a trusted EpochProtocolStateAdapter using NewEpochProtocolStateAdapter constructor.
type UntrustedEpochProtocolStateAdapter EpochProtocolStateAdapter

// NewEpochProtocolStateAdapter creates a new instance of EpochProtocolStateAdapter.
// Construction EpochProtocolStateAdapter allowed only within the constructor.
//
// All errors indicate a valid EpochProtocolStateAdapter cannot be constructed from the input.
func NewEpochProtocolStateAdapter(untrusted UntrustedEpochProtocolStateAdapter) (*EpochProtocolStateAdapter, error) {
	if untrusted.Params == nil {
		return nil, fmt.Errorf("params must not be nil")
	}
	if untrusted.RichEpochStateEntry == nil {
		return nil, fmt.Errorf("rich epoch state must not be nil")
	}
	return &EpochProtocolStateAdapter{
		RichEpochStateEntry: untrusted.RichEpochStateEntry,
		Params:              untrusted.Params,
	}, nil
}

// Epoch returns the current epoch counter.
func (s *EpochProtocolStateAdapter) Epoch() uint64 {
	return s.CurrentEpochSetup.Counter
}

// Clustering returns the cluster assignment for the current epoch.
// No errors are expected during normal operations.
func (s *EpochProtocolStateAdapter) Clustering() (flow.ClusterList, error) {
	clustering, err := ClusteringFromSetupEvent(s.CurrentEpochSetup)
	if err != nil {
		return nil, fmt.Errorf("could not extract cluster list from setup event: %w", err)
	}
	return clustering, nil
}

// EpochSetup returns the flow.EpochSetup service event which partly defines the
// initial epoch state for the current epoch.
func (s *EpochProtocolStateAdapter) EpochSetup() *flow.EpochSetup {
	return s.CurrentEpochSetup
}

// EpochCommit returns the flow.EpochCommit service event which partly defines the
// initial epoch state for the current epoch.
func (s *EpochProtocolStateAdapter) EpochCommit() *flow.EpochCommit {
	return s.CurrentEpochCommit
}

// DKG returns the DKG information for the current epoch.
// No errors are expected during normal operations.
func (s *EpochProtocolStateAdapter) DKG() (protocol.DKG, error) {
	return NewDKG(s.CurrentEpochSetup, s.CurrentEpochCommit), nil
}

// Entry Returns low-level protocol state entry that was used to initialize this object.
// It shouldn't be used by high-level logic, it is useful for some cases such as bootstrapping.
// Prefer using other methods to access protocol state.
func (s *EpochProtocolStateAdapter) Entry() *flow.RichEpochStateEntry {
	return s.RichEpochStateEntry.Copy()
}

// Identities returns the identity table as of the current block.
func (s *EpochProtocolStateAdapter) Identities() flow.IdentityList {
	return s.RichEpochStateEntry.CurrentEpochIdentityTable
}

// GlobalParams returns spork-scoped global network parameters.
func (s *EpochProtocolStateAdapter) GlobalParams() protocol.GlobalParams {
	return s.Params
}

// EpochFallbackTriggered denotes whether an invalid epoch state transition was attempted
// on the fork ending this block. Once the first block where this flag is true is finalized, epoch
// fallback mode is triggered.
// TODO for 'leaving Epoch Fallback via special service event': at the moment, this is a one-way transition and requires a spork to recover - need to revisit for sporkless EFM recovery
func (s *EpochProtocolStateAdapter) EpochFallbackTriggered() bool {
	return s.MinEpochStateEntry.EpochFallbackTriggered
}

// PreviousEpochExists returns true if a previous epoch exists. This is true for all epoch
// except those immediately following a spork.
func (s *EpochProtocolStateAdapter) PreviousEpochExists() bool {
	return s.PreviousEpoch != nil
}

// EpochExtensions returns the epoch extensions associated with the current epoch, if any.
func (s *EpochProtocolStateAdapter) EpochExtensions() []flow.EpochExtension {
	return s.CurrentEpoch.EpochExtensions
}

// EpochPhase returns the epoch phase for the current epoch.
// The receiver must be properly constructed.
// See flow.EpochPhase for detailed documentation.
func (s *EpochProtocolStateAdapter) EpochPhase() flow.EpochPhase {
	return s.Entry().EpochPhase()
}
