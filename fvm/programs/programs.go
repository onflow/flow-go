package programs

import (
	"sync"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/state"
)

// TODO(patrick): rm after https://github.com/onflow/flow-emulator/pull/229
// is merged and integrated.
type Programs struct {
	lock sync.RWMutex

	block      *derived.DerivedBlockData
	currentTxn *derived.DerivedTransactionData

	logicalTime derived.LogicalTime
}

func NewEmptyPrograms() *Programs {
	block := derived.NewEmptyDerivedBlockData()
	txn, err := block.NewDerivedTransactionData(0, 0)
	if err != nil {
		panic(err)
	}

	return &Programs{
		block:       block,
		currentTxn:  txn,
		logicalTime: 0,
	}
}

func (p *Programs) ChildPrograms() *Programs {
	p.lock.RLock()
	defer p.lock.RUnlock()

	childBlock := p.block.NewChildDerivedBlockData()
	txn, err := childBlock.NewDerivedTransactionData(0, 0)
	if err != nil {
		panic(err)
	}

	return &Programs{
		block:       childBlock,
		currentTxn:  txn,
		logicalTime: 0,
	}
}

func (p *Programs) NextTxIndexForTestingOnly() uint32 {
	return p.block.NextTxIndexForTestingOnly()
}

func (p *Programs) GetForTestingOnly(location common.AddressLocation) (*derived.Program, *state.State, bool) {
	return p.GetProgram(location)
}

func (p *Programs) GetProgram(location common.AddressLocation) (*derived.Program, *state.State, bool) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return p.currentTxn.GetProgram(location)
}

func (p *Programs) SetProgram(location common.AddressLocation, program *derived.Program, state *state.State) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	p.currentTxn.SetProgram(location, program, state)
}

func (p *Programs) Cleanup(invalidator derived.TransactionInvalidator) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.currentTxn.AddInvalidator(invalidator)

	var err error
	err = p.currentTxn.Commit()
	if err != nil {
		panic(err)
	}

	p.logicalTime++
	txn, err := p.block.NewDerivedTransactionData(p.logicalTime, p.logicalTime)
	if err != nil {
		panic(err)
	}

	p.currentTxn = txn
}
