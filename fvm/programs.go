package fvm

import (
	"sync"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
)

type ProgramEntry struct {
	Location common.Location
	Program *interpreter.Program
}

type Programs struct {
	lock     sync.RWMutex
	parent   *Programs
	programs map[common.LocationID]ProgramEntry
}

func NewPrograms(parent *Programs) *Programs {
	return &Programs{
		parent: parent,
		programs: map[common.LocationID]ProgramEntry{},
	}
}

func (p *Programs) Get(locationID common.LocationID) *interpreter.Program {
	p.lock.RLock()
	defer p.lock.RUnlock()

	// First check if the program is available here
	if programEntry, ok := p.programs[locationID]; ok {
		return programEntry.Program
	}

	// Second, check if the program is available in the parent (if any)

	if p.parent != nil {
		return p.parent.Get(locationID)
	}

	// The program is neither available here nor in the parent

	return nil
}

func (p *Programs) Set(location common.Location, program *interpreter.Program) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.programs[location.ID()] = ProgramEntry{
		Location: location,
		Program: program,
	}
}

// Commit sets all programs into the parent (if any)
//
func (p *Programs) Commit() {
	p.lock.Lock()
	defer p.lock.Unlock()

	parent := p.parent

	if parent == nil {
		return
	}

	for _, programEntry := range p.programs {

		// Only commit programs with address locations,
		// i.e. programs for contracts,
		// and not programs for transactions or scripts

		if _, ok := programEntry.Location.(common.AddressLocation); !ok {
			continue
		}

		parent.Set(
			programEntry.Location,
			programEntry.Program,
		)
	}
}

func (p *Programs) Rollback() {
	p.lock.Lock()
	defer p.lock.Unlock()

	// Clear all program entries

	for locationID := range p.programs {
		delete(p.programs, locationID)
	}
}

func (p *Programs) ForEach(fn func(location common.Location, program *interpreter.Program)) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	for _, programEntry := range p.programs {
		fn(
			programEntry.Location,
			programEntry.Program,
		)
	}
}
