package fvm

import (
	"sync"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm/handler"
)

type ProgramEntry struct {
	Location common.Location
	Program  *interpreter.Program
	View     *delta.View
}

type ProgramGetFunc func(location common.Location) (*ProgramEntry, bool)

func emptyProgramGetFunc(_ common.Location) (*ProgramEntry, bool) {
	return nil, false
}

// Programs is a cumulative cache-like storage for Programs helping speed up execution of Cadence
// Programs don't evict elements at will, like a typical cache would, but it does it only
// during a cleanup method, which must be called only when the Cadence execution has finished.
// It it also fork-aware, support cheap creation of children capturing local changes.
type Programs struct {
	lock       sync.RWMutex
	programs   map[common.LocationID]ProgramEntry
	parentFunc ProgramGetFunc
	cleaned    bool
}

func NewEmptyPrograms() *Programs {

	return &Programs{
		programs:   map[common.LocationID]ProgramEntry{},
		parentFunc: emptyProgramGetFunc,
	}
}

func (p *Programs) ChildPrograms() *Programs {
	return &Programs{
		programs: map[common.LocationID]ProgramEntry{},
		parentFunc: func(location common.Location) (*ProgramEntry, bool) {
			return p.get(location)
		},
	}
}

func (p *Programs) Get(location common.Location) (*interpreter.Program, *delta.View, bool) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	programEntry, has := p.get(location)

	if has {
		return programEntry.Program, programEntry.View, true
	}

	return nil, nil, false
}

func (p *Programs) get(location common.Location) (*ProgramEntry, bool) {
	programEntry, ok := p.programs[location.ID()]
	if !ok {
		parentEntry, has := p.parentFunc(location)

		if has {
			return parentEntry, true
		}
		return nil, false
	}
	return &programEntry, true
}

func (p *Programs) Set(location common.Location, program *interpreter.Program, view *delta.View) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.programs[location.ID()] = ProgramEntry{
		Location: location,
		Program:  program,
		View:     view,
	}
}

// HasChanges indicates if any changes has been introduced
// essentially telling if this object is identical to its parent
func (p *Programs) HasChanges() bool {
	return len(p.programs) > 0 || p.cleaned
}

// ForceCleanup is used to force a complete cleanup
// It exists temporarily to facilitate a temporary measure which can retry
// a transaction in case checking fails
// It should be gone when the extra retry is gone
func (p *Programs) ForceCleanup() {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.cleaned = true

	// Stop using parent's data to prevent
	// infinite chaining of objects
	p.parentFunc = emptyProgramGetFunc

	// start with empty storage
	p.programs = make(map[common.LocationID]ProgramEntry)
}

func (p *Programs) Cleanup(changedContracts []handler.ContractUpdateKey) {
	p.lock.Lock()
	defer p.lock.Unlock()

	// In mature system, we would track dependencies between contracts
	// and invalidate only affected ones, possibly setting them to
	// nil so they will override parent's data, but for now
	// just throw everything away and use a special flag for this
	if len(changedContracts) > 0 {

		p.cleaned = true

		// Sop using parent's data to prevent
		// infinite chaining of objects
		p.parentFunc = emptyProgramGetFunc

		// start with empty storage
		p.programs = make(map[common.LocationID]ProgramEntry)
		return
	}

	// However, if none of the programs were changed
	// we remove all the non AddressLocation data
	// (those are temporary tx related entries)
	for id, entry := range p.programs {
		if _, is := entry.Location.(common.AddressLocation); !is {
			delete(p.programs, id)
		}
	}
}
