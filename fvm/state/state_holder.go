package state

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/model/flow"
)

type nestedTransactionStackFrame struct {
	state *State

	// When nil, the subtransaction will have unrestricted access to the runtime
	// environment.  When non-nil, the subtransaction will only have access to
	// the parts of the runtime environment necessary for importing/parsing the
	// program, specifically, environment.ContractReader and
	// environment.Programs.
	//
	// TODO(patrick): restrict environment method access
	parseRestriction *common.AddressLocation
}

// TODO(patrick): rename to StateTransaction
// StateHolder provides active states
// and facilitates common state management operations
// in order to make services such as accounts not worry about
// the state it is recommended that such services wraps
// a state manager instead of a state itself.
type StateHolder struct {
	enforceLimits bool

	// NOTE: The first frame is always the main transaction, and is not
	// poppable during the course of the transaction.
	nestedTransactions []nestedTransactionStackFrame
}

// Opaque identifier used for Restarting nested transactions
type NestedTransactionId struct {
	state *State
}

func (id NestedTransactionId) StateForTestingOnly() *State {
	return id.state
}

// NewStateTransaction constructs a new state transaction which manages nested
// transactions.
func NewStateTransaction(startView View, params StateParameters) *StateHolder {
	startState := NewState(startView, params)
	return &StateHolder{
		enforceLimits: true,
		nestedTransactions: []nestedTransactionStackFrame{
			nestedTransactionStackFrame{
				state:            startState,
				parseRestriction: nil,
			},
		},
	}
}

func (s *StateHolder) current() nestedTransactionStackFrame {
	return s.nestedTransactions[s.NumNestedTransactions()]
}

func (s *StateHolder) currentState() *State {
	return s.current().state
}

// NumNestedTransactions returns the number of uncommitted nested transactions.
// Note that the main transaction is not considered a nested transaction.
func (s *StateHolder) NumNestedTransactions() int {
	return len(s.nestedTransactions) - 1
}

// IsParseRestricted returns true if the current nested transaction is in
// parse resticted access mode.
func (s *StateHolder) IsParseRestricted() bool {
	return s.current().parseRestriction != nil
}

func (s *StateHolder) MainTransactionId() NestedTransactionId {
	return NestedTransactionId{
		state: s.nestedTransactions[0].state,
	}
}

// IsCurrent returns true if the provide id refers to the current (nested)
// transaction.
func (s *StateHolder) IsCurrent(id NestedTransactionId) bool {
	return s.currentState() == id.state
}

// BeginNestedTransaction creates a unrestricted nested transaction within the
// current unrestricted (nested) transaction.  This returns error if the current
// nested transaction is program restricted.
func (s *StateHolder) BeginNestedTransaction() (NestedTransactionId, error) {
	if s.IsParseRestricted() {
		return NestedTransactionId{}, fmt.Errorf(
			"cannot beinga a unrestricted nested transaction inside a " +
				"program restricted nested transaction",
		)
	}

	child := s.currentState().NewChild()

	s.nestedTransactions = append(
		s.nestedTransactions,
		nestedTransactionStackFrame{
			state:            child,
			parseRestriction: nil,
		},
	)

	return NestedTransactionId{
		state: child,
	}, nil
}

// BeginParseRestrictedNestedTransaction creates a restricted nested
// transaction within the current (nested) transaction.
func (s *StateHolder) BeginParseRestrictedNestedTransaction(
	location common.AddressLocation,
) (
	NestedTransactionId,
	error,
) {
	child := s.currentState().NewChild()

	s.nestedTransactions = append(
		s.nestedTransactions,
		nestedTransactionStackFrame{
			state:            child,
			parseRestriction: &location,
		},
	)

	return NestedTransactionId{
		state: child,
	}, nil
}

func (s *StateHolder) mergeIntoParent() error {
	if len(s.nestedTransactions) < 2 {
		return fmt.Errorf("cannot commit the main transaction")
	}

	child := s.current()
	s.nestedTransactions = s.nestedTransactions[:len(s.nestedTransactions)-1]
	parent := s.current()

	return parent.state.MergeState(child.state)
}

// Commit commits the changes in the current unrestricted nested transaction
// to the parent (nested) transaction.  This returns error if the expectedId
// does not match the current nested transaction.
func (s *StateHolder) Commit(
	expectedId NestedTransactionId,
) error {
	if !s.IsCurrent(expectedId) {
		return fmt.Errorf(
			"cannot commit unexpected nested transaction: id mismatch",
		)
	}

	if s.IsParseRestricted() {
		// This is due to a programming error.
		return fmt.Errorf(
			"cannot commit unexpected nested transaction: parse restricted",
		)
	}

	return s.mergeIntoParent()
}

// CommitParseRestricted commits the changes in the current restricted nested
// transaction to the parent (nested) transaction.  This returns error if the
// specified location does not match the tracked location.
func (s *StateHolder) CommitParseRestricted(
	location common.AddressLocation,
) (
	*State,
	error,
) {
	currentFrame := s.current()
	if currentFrame.parseRestriction == nil ||
		*currentFrame.parseRestriction != location {

		// This is due to a programming error.
		return nil, fmt.Errorf(
			"cannot commit unexpected nested transaction %v != %v",
			currentFrame.parseRestriction,
			location,
		)
	}

	err := s.mergeIntoParent()
	if err != nil {
		return nil, err
	}

	return currentFrame.state, nil
}

// AttachAndCommitParseRestricted commits the changes in the cached nested
// transaction to the current (nested) transaction.
func (s *StateHolder) AttachAndCommitParseRestricted(
	cachedNestedTransaction *State,
) error {
	s.nestedTransactions = append(
		s.nestedTransactions,
		nestedTransactionStackFrame{
			state:            cachedNestedTransaction,
			parseRestriction: nil,
		},
	)

	return s.mergeIntoParent()
}

// RestartNestedTransaction merges all changes that belongs to the nested
// transaction about to be restart (for spock/meter bookkeeping), then
// wipes its view changes.
func (s *StateHolder) RestartNestedTransaction(
	id NestedTransactionId,
) error {

	// NOTE: We need to verify the id is valid before any merge operation or
	// else we would accidently merge everything into the main transaction.
	found := false
	for _, frame := range s.nestedTransactions {
		if frame.state == id.state {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("nested transaction not found")
	}

	for s.currentState() != id.state {
		err := s.mergeIntoParent()
		if err != nil {
			return fmt.Errorf("cannot restart nested transaction: %w", err)
		}
	}

	s.currentState().View().DropDelta()
	return nil
}

func (s *StateHolder) Get(
	owner string,
	key string,
	enforceLimit bool,
) (
	flow.RegisterValue,
	error,
) {
	return s.currentState().Get(owner, key, enforceLimit)
}

func (s *StateHolder) Set(
	owner string,
	key string,
	value flow.RegisterValue,
	enforceLimit bool,
) error {
	return s.currentState().Set(owner, key, value, enforceLimit)
}

func (s *StateHolder) UpdatedAddresses() []flow.Address {
	return s.currentState().UpdatedAddresses()
}

func (s *StateHolder) MeterComputation(
	kind common.ComputationKind,
	intensity uint,
) error {
	return s.currentState().MeterComputation(kind, intensity)
}

func (s *StateHolder) MeterMemory(
	kind common.MemoryKind,
	intensity uint,
) error {
	return s.currentState().MeterMemory(kind, intensity)
}

func (s *StateHolder) ComputationIntensities() meter.MeteredComputationIntensities {
	return s.currentState().ComputationIntensities()
}

func (s *StateHolder) TotalComputationLimit() uint {
	return s.currentState().TotalComputationLimit()
}

func (s *StateHolder) TotalComputationUsed() uint {
	return s.currentState().TotalComputationUsed()
}

func (s *StateHolder) MemoryIntensities() meter.MeteredMemoryIntensities {
	return s.currentState().MemoryIntensities()
}

func (s *StateHolder) TotalMemoryEstimate() uint64 {
	return s.currentState().TotalMemoryEstimate()
}

func (s *StateHolder) InteractionUsed() uint64 {
	return s.currentState().InteractionUsed()
}

func (s *StateHolder) ViewForTestingOnly() View {
	return s.currentState().View()
}

// EnableAllLimitEnforcements enables all the limits
func (s *StateHolder) EnableAllLimitEnforcements() {
	s.enforceLimits = true
}

// DisableAllLimitEnforcements disables all the limits
func (s *StateHolder) DisableAllLimitEnforcements() {
	s.enforceLimits = false
}

// RunWithAllLimitsDisabled runs f with limits disabled
func (s *StateHolder) RunWithAllLimitsDisabled(f func()) {
	if f == nil {
		return
	}
	current := s.enforceLimits
	s.enforceLimits = false
	f()
	s.enforceLimits = current
}

// EnforceComputationLimits returns if the computation limits should be enforced
// or not.
func (s *StateHolder) EnforceComputationLimits() bool {
	return s.enforceLimits
}

// EnforceInteractionLimits returns if the interaction limits should be enforced or not
func (s *StateHolder) EnforceLimits() bool {
	return s.enforceLimits
}
