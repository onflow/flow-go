package programs

import (
	"sync"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-go/fvm/state"
)

type ContractUpdateKey struct {
	Address flow.Address
	Name    string
}

type ModifiedSets struct {
	ContractUpdateKeys []ContractUpdateKey
	FrozenAccounts     []common.Address
}

type ContractUpdate struct {
	ContractUpdateKey
	Code []byte
}

type ProgramEntry struct {
	Location common.Location
	Program  *interpreter.Program
	State    *state.State
}

// Programs is a cumulative cache-like storage for Programs helping speed up execution of Cadence
// Programs don't evict elements at will, like a typical cache would, but it does it only
// during a cleanup method, which must be called only when the Cadence execution has finished.
// It it also fork-aware, support cheap creation of children capturing local changes.
type Programs struct {
	lock     sync.RWMutex
	programs map[common.LocationID]*ProgramEntry
	parent   *Programs
	cleaned  bool
}

func NewEmptyPrograms() *Programs {
	return &Programs{
		programs: map[common.LocationID]*ProgramEntry{},
	}
}

func (p *Programs) ChildPrograms() *Programs {
	return &Programs{
		programs: map[common.LocationID]*ProgramEntry{},
		parent:   p,
	}
}

// Get returns stored program, state which contains changes which correspond to loading this program,
// and boolean indicating if the value was found
func (p *Programs) Get(location common.AddressLocation) (*interpreter.Program, *state.State, bool) {
	entry, parent := p.get(location)
	if entry != nil {
		return entry.Program, entry.State, true
	}

	if parent != nil {
		return parent.Get(location)
	}

	return nil, nil, false
}

func (p *Programs) get(location common.AddressLocation) (*ProgramEntry, *Programs) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return p.programs[location.ID()], p.parent
}

func (p *Programs) Set(location common.AddressLocation, program *interpreter.Program, state *state.State) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.programs[location.ID()] = &ProgramEntry{
		Location: location,
		Program:  program,
		State:    state,
	}
}

// HasChanges indicates if any changes has been introduced
// essentially telling if this object is identical to its parent
func (p *Programs) HasChanges() bool {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return len(p.programs) > 0 || p.cleaned
}

func (p *Programs) Cleanup(modifiedSets ModifiedSets) {
	p.lock.Lock()
	defer p.lock.Unlock()

	// In mature system, we would track dependencies between contracts
	// and invalidate only affected ones, possibly setting them to
	// nil so they will override parent's data, but for now
	// just throw everything away and use a special flag for this
	if len(modifiedSets.ContractUpdateKeys) > 0 ||
		len(modifiedSets.FrozenAccounts) > 0 {

		p.cleaned = true

		// Stop using parent's data to prevent
		// infinite chaining of objects
		p.parent = nil

		// start with empty storage
		p.programs = make(map[common.LocationID]*ProgramEntry)
		return
	}
}
