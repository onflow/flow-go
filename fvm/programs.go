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

type ProgramGetFunc func(location common.Location) *ProgramEntry

func emptyProgramGetFunc(location common.Location) *ProgramEntry {
	return nil
}

type Programs struct {
	lock       sync.RWMutex
	programs   map[common.LocationID]ProgramEntry
	parentFunc ProgramGetFunc
}

func NewEmptyPrograms() *Programs {

	return &Programs{
		programs:   map[common.LocationID]ProgramEntry{},
		parentFunc: emptyProgramGetFunc,
	}
}

func (p *Programs) ChildPrograms() *Programs {
	return &Programs{
		programs:   map[common.LocationID]ProgramEntry{},
		parentFunc: p.parentFunc,
	}
}

func (p *Programs) Get(location common.Location) *interpreter.Program {
	p.lock.RLock()
	defer p.lock.RUnlock()

	programEntry, ok := p.programs[location.ID()]
	if !ok {

		parentEntry := p.parentFunc(location)

		if parentEntry != nil {
			return parentEntry.Program
		}

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

func (p *Programs) Cleanup(willDiscard bool) {
	p.lock.Lock()
	defer p.lock.Unlock()

	// In mature system, we would track dependencies between contracts
	// and invalidate only affected ones
	// But currently we just stop using parent's data to prevent
	// infinite chaining of objects
	p.parentFunc = emptyProgramGetFunc

	// additionally, if we are not discarding this cache
	// we remove all the non AddressLocation data
	// (those are temporary tx related entries)
	// if cache is going to be discarded anyway
	// we don't bother
	if !willDiscard {
		for id, entry := range p.programs {
			if _, is := entry.Location.(common.AddressLocation); !is {
				delete(p.programs, id)
			}
		}
	}
}
