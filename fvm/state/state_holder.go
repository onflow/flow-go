package state

import (
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/model/flow"
)

// TODO(patrick): rename to StateTransaction
// StateHolder provides active states
// and facilitates common state management operations
// in order to make services such as accounts not worry about
// the state it is recommended that such services wraps
// a state manager instead of a state itself.
type StateHolder struct {
	enforceLimits         bool
	payerIsServiceAccount bool
	startState            *State
	activeState           *State
}

// NewStateHolder constructs a new state manager
func NewStateHolder(startState *State) *StateHolder {
	return &StateHolder{
		enforceLimits: true,
		startState:    startState,
		activeState:   startState,
	}
}

// TODO(patrick): remove
// State returns the active state
func (s *StateHolder) State() *State {
	return s.activeState
}

// TODO(patrick): remove
// SetActiveState sets active state
func (s *StateHolder) SetActiveState(st *State) {
	s.activeState = st
}

func (s *StateHolder) Get(
	owner string,
	key string,
	enforceLimit bool,
) (
	flow.RegisterValue,
	error,
) {
	return s.activeState.Get(owner, key, enforceLimit)
}

func (s *StateHolder) Set(
	owner string,
	key string,
	value flow.RegisterValue,
	enforceLimit bool,
) error {
	return s.activeState.Set(owner, key, value, enforceLimit)
}

func (s *StateHolder) UpdatedAddresses() []flow.Address {
	return s.activeState.UpdatedAddresses()
}

// TODO(patrick): remove once we no longer support updating limit/weights after
// meter initialization.
func (s *StateHolder) Meter() meter.Meter {
	return s.activeState.Meter()
}

func (s *StateHolder) MeterComputation(
	kind common.ComputationKind,
	intensity uint,
) error {
	return s.activeState.MeterComputation(kind, intensity)
}

func (s *StateHolder) MeterMemory(
	kind common.MemoryKind,
	intensity uint,
) error {
	return s.activeState.MeterMemory(kind, intensity)
}

func (s *StateHolder) ComputationIntensities() meter.MeteredComputationIntensities {
	return s.activeState.ComputationIntensities()
}

func (s *StateHolder) TotalComputationLimit() uint {
	return s.activeState.TotalComputationLimit()
}

func (s *StateHolder) TotalComputationUsed() uint {
	return s.activeState.TotalComputationUsed()
}

func (s *StateHolder) MemoryIntensities() meter.MeteredMemoryIntensities {
	return s.activeState.MemoryIntensities()
}

func (s *StateHolder) TotalMemoryEstimate() uint {
	return s.activeState.TotalMemoryEstimate()
}

func (s *StateHolder) InteractionUsed() uint64 {
	return s.activeState.InteractionUsed()
}

func (s *StateHolder) ViewForTestingOnly() View {
	return s.activeState.View()
}

// SetPayerIsServiceAccount sets if the payer is the service account
func (s *StateHolder) SetPayerIsServiceAccount() {
	s.payerIsServiceAccount = true
}

// NewChild constructs a new child of active state
// and set it as active state and return it
// this is basically a utility function for common
// operations
func (s *StateHolder) NewChild() *State {
	child := s.activeState.NewChild()
	s.activeState = child
	return s.activeState
}

// EnableAllLimitEnforcements enables all the limits
func (s *StateHolder) EnableAllLimitEnforcements() {
	s.enforceLimits = true
}

// DisableAllLimitEnforcements disables all the limits
func (s *StateHolder) DisableAllLimitEnforcements() {
	s.enforceLimits = false
}

// EnforceComputationLimits returns if the computation limits should be enforced
// or not.
func (s *StateHolder) EnforceComputationLimits() bool {
	return s.enforceLimits
}

// EnforceInteractionLimits returns if the interaction limits should be enforced or not
func (s *StateHolder) EnforceInteractionLimits() bool {
	return !s.payerIsServiceAccount && s.enforceLimits
}

// EnforceMemoryLimits returns if the memory limits should be enforced or not
func (s *StateHolder) EnforceMemoryLimits() bool {
	return !s.payerIsServiceAccount && s.enforceLimits
}
