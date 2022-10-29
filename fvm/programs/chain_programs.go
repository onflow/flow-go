package programs

import (
	"fmt"
	"sync"

	"github.com/hashicorp/golang-lru/simplelru"

	"github.com/onflow/flow-go/model/flow"
)

const DefaultProgramsCacheSize = 1000

type BlockRuntimeItem struct {
	Programs      *BlockPrograms
	MeterSettings *BlockMeterSettings
}

// ChainPrograms is a cache of BlockPrograms databases used for speeding up
// cadence execution.
//
// Since programs are derived from external source, the BlockPrograms databases
// need not be durable and can be recreated on the fly.
type ChainPrograms struct {
	// NOTE: It's unsafe to use RWMutex since lru updates the data structure
	// on Get.
	mutex sync.Mutex

	lru *simplelru.LRU
}

func NewChainPrograms(chainCacheSize uint) (*ChainPrograms, error) {
	lru, err := simplelru.NewLRU(int(chainCacheSize), nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create LRU cache: %w", err)
	}

	return &ChainPrograms{
		lru: lru,
	}, nil
}

func (chain *ChainPrograms) unsafeGet(
	currentBlockId flow.Identifier,
) *BlockRuntimeItem {
	currentEntry, ok := chain.lru.Get(currentBlockId)
	if ok {
		return currentEntry.(*BlockRuntimeItem)
	}

	return nil
}

func (chain *ChainPrograms) Get(
	currentBlockId flow.Identifier,
) *BlockRuntimeItem {
	chain.mutex.Lock()
	defer chain.mutex.Unlock()

	return chain.unsafeGet(currentBlockId)
}

func (chain *ChainPrograms) GetOrCreateBlockPrograms(
	currentBlockId flow.Identifier,
	parentBlockId flow.Identifier,
) *BlockRuntimeItem {
	chain.mutex.Lock()
	defer chain.mutex.Unlock()

	currentEntry := chain.unsafeGet(currentBlockId)
	if currentEntry != nil {
		return currentEntry
	}

	var currentPrograms *BlockPrograms
	var currentMeterSettings *BlockMeterSettings

	parentEntry, ok := chain.lru.Get(parentBlockId)
	if ok {
		currentPrograms = parentEntry.(*BlockRuntimeItem).Programs.NewChildOCCBlock()
		currentMeterSettings = parentEntry.(*BlockRuntimeItem).MeterSettings.NewChildBlockMeterSettings()
	} else {
		currentPrograms = NewEmptyBlockPrograms()
		currentMeterSettings = NewBlockMeterSettings()
	}

	current := &BlockRuntimeItem{
		Programs:      currentPrograms,
		MeterSettings: currentMeterSettings,
	}

	chain.lru.Add(currentBlockId, *current)
	return current
}

func (chain *ChainPrograms) NewBlockRuntimeItemForScript(
	currentBlockId flow.Identifier,
) *BlockRuntimeItem {
	block := chain.Get(currentBlockId)
	if block != nil {
		return &BlockRuntimeItem{
			Programs:      block.Programs.NewChildOCCBlock(),
			MeterSettings: block.MeterSettings.NewChildBlockMeterSettings(),
		}
	}

	return &BlockRuntimeItem{}
}
