package programs

import (
	"sync"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/state"
)

// TODO(patrick): Remove after emulator is updated.
type Programs struct {
	lock sync.RWMutex

	block      *BlockPrograms
	currentTxn *TransactionPrograms

	logicalTime LogicalTime
}

func NewEmptyPrograms() *Programs {
	block := NewEmptyBlockPrograms()
	txn, err := block.NewOCCBlockItem(0, 0)
	if err != nil {
		panic(err)
	}

	return &Programs{
		block: block,
		currentTxn: &TransactionPrograms{
			transactionPrograms: *txn,
		},
		logicalTime: 0,
	}
}

func (p *Programs) ChildPrograms() *Programs {
	p.lock.RLock()
	defer p.lock.RUnlock()

	childBlock := p.block.NewChildOCCBlock()
	txn, err := childBlock.NewOCCBlockItem(0, 0)
	if err != nil {
		panic(err)
	}

	return &Programs{
		block: childBlock,
		currentTxn: &TransactionPrograms{
			transactionPrograms: *txn,
		},
		logicalTime: 0,
	}
}

func (p *Programs) NextTxIndexForTestingOnly() uint32 {
	return p.block.NextTxIndexForTestingOnly()
}

func (p *Programs) GetForTestingOnly(location common.Location) *ProgramEntry {
	return p.Get(location)
}

// Get returns stored program, state which contains changes which correspond to loading this program,
// and boolean indicating if the value was found
func (p *Programs) Get(location common.Location) *ProgramEntry {
	p.lock.RLock()
	defer p.lock.RUnlock()

	program, state, ok := p.currentTxn.Get(location)
	if !ok {
		return nil
	}

	return &ProgramEntry{
		Program: program,
		State:   state,
	}
}

func (p *Programs) Set(location common.Location, program *interpreter.Program, state *state.State) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	p.currentTxn.Set(location, program, state)
}

func (p *Programs) Cleanup(modifiedSets OCCProgramsInvalidator) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.currentTxn.transactionPrograms.AddInvalidator(modifiedSets)

	var err error
	err = p.currentTxn.transactionPrograms.Commit()
	if err != nil {
		panic(err)
	}

	p.logicalTime++
	txn, err := p.block.NewOCCBlockItem(p.logicalTime, p.logicalTime)
	if err != nil {
		panic(err)
	}

	p.currentTxn = &TransactionPrograms{
		transactionPrograms: *txn,
	}
}
