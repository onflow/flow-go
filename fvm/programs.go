package fvm

import (
	"sync"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
)

type Programs struct {
	lock sync.RWMutex
	programs map[common.LocationID]*interpreter.Program
}

func NewPrograms() *Programs {
	return &Programs{
		programs: map[common.LocationID]*interpreter.Program{},
	}
}

func (p *Programs) Get(locationID common.LocationID) *interpreter.Program {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return p.programs[locationID]
}

func (p *Programs) Set(locationID common.LocationID, program *interpreter.Program) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.programs[locationID] = program
}
