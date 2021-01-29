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
	programs map[common.LocationID]ProgramEntry
	draft    map[common.LocationID]ProgramEntry
}

func NewPrograms() *Programs {
	return &Programs{
		programs: map[common.LocationID]ProgramEntry{},
		draft: map[common.LocationID]ProgramEntry{},
	}
}

func (p *Programs) Get(locationID common.LocationID) *interpreter.Program {
	p.lock.RLock()
	defer p.lock.RUnlock()

	// Check is the program is in the drafts
	programEntry, ok := p.draft[locationID]
	if ok {
		return programEntry.Program
	}

	return p.programs[locationID].Program
}

func (p *Programs) Set(location common.Location, program *interpreter.Program) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.draft[location.ID()] = ProgramEntry{
		Location: location,
		Program: program,
	}
}

func (p *Programs) Commit() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	// Commit all drafts

	for locationID, programEntry := range p.draft {

		// Only commit programs with address locations,
		// i.e. programs for contracts,
		// and not programs for transactions or scripts

		if _, ok := programEntry.Location.(common.AddressLocation); !ok {
			continue
		}

		p.programs[locationID] = programEntry
	}

	// Clear drafts

	for locationID := range p.draft {
		delete(p.draft, locationID)
	}

	return nil
}

func (p *Programs) Rollback() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	// Clear drafts

	for locationID := range p.draft {
		delete(p.draft, locationID)
	}

	return nil
}
