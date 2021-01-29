package fvm

import (
	"sync"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
)

type Programs struct {
	lock     sync.RWMutex
	programs map[common.LocationID]*interpreter.Program
	draft    map[common.LocationID]*interpreter.Program
}

func NewPrograms() *Programs {
	return &Programs{
		programs: map[common.LocationID]*interpreter.Program{},
	}
}

func (p *Programs) Get(locationID common.LocationID) *interpreter.Program {
	p.lock.RLock()
	defer p.lock.RUnlock()

	// check draft first
	if prog, ok := p.draft[locationID]; ok {
		return prog
	}

	return p.programs[locationID]
}

func (p *Programs) Set(locationID common.LocationID, program *interpreter.Program) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.draft[locationID] = program
}

func (p *Programs) Commit() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	for loc, prog := range p.draft {
		if _, ok := loc.(common.AddressLocation); !ok {
			// ignore cases that are not addresslocation
			continue
		}
		// TODO make me smarter
		// if loc already exist this is an update - purge the cache
		if _, ok := p.programs[loc]; ok {
			p.programs = make(map[common.LocationID]*interpreter.Program)
			break
		}
		p.programs[loc] = prog
	}

	// reset draft
	p.draft = make(map[common.LocationID]*interpreter.Program)
	return nil
}

func (p *Programs) Rollback() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	// reset draft
	p.draft = make(map[common.LocationID]*interpreter.Program)
	return nil
}
