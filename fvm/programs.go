package fvm

import (
	"sync"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
)

type ProgramEntry struct {
	Location common.Location
	Program  *interpreter.Program
}

type Programs struct {
	lock     sync.RWMutex
	programs map[common.LocationID]ProgramEntry
}

func NewPrograms() *Programs {
	return &Programs{
		programs: map[common.LocationID]ProgramEntry{},
	}
}

func (p *Programs) Get(location common.Location) *interpreter.Program {
	p.lock.RLock()
	defer p.lock.RUnlock()

	programEntry, ok := p.programs[location.ID()]
	if !ok {
		return nil
	}

	return programEntry.Program
}

func (p *Programs) Set(location common.Location, program *interpreter.Program) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.programs[location.ID()] = ProgramEntry{
		Location: location,
		Program:  program,
	}
}

