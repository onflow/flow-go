package derived

import (
	"fmt"
	"sync"

	"github.com/hashicorp/golang-lru/v2/simplelru"

	"github.com/onflow/flow-go/model/flow"
)

const DefaultDerivedDataCacheSize = 1000

// DerivedChainData is a cache of DerivedBlockData databases used for speeding up
// cadence execution.
//
// Since programs are derived from external source, the DerivedBlockData databases
// need not be durable and can be recreated on the fly.
type DerivedChainData struct {
	// NOTE: It's unsafe to use RWMutex since lru updates the data structure
	// on Get.
	mutex sync.Mutex

	lru *simplelru.LRU[flow.Identifier, *DerivedBlockData]
}

func NewDerivedChainData(chainCacheSize uint) (*DerivedChainData, error) {
	lru, err := simplelru.NewLRU[flow.Identifier, *DerivedBlockData](int(chainCacheSize), nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create LRU cache: %w", err)
	}

	return &DerivedChainData{
		lru: lru,
	}, nil
}

func (chain *DerivedChainData) unsafeGet(
	currentBlockId flow.Identifier,
) *DerivedBlockData {
	currentEntry, ok := chain.lru.Get(currentBlockId)
	if ok {
		return currentEntry
	}

	return nil
}

func (chain *DerivedChainData) Get(
	currentBlockId flow.Identifier,
) *DerivedBlockData {
	chain.mutex.Lock()
	defer chain.mutex.Unlock()

	return chain.unsafeGet(currentBlockId)
}

func (chain *DerivedChainData) GetOrCreateDerivedBlockData(
	currentBlockId flow.Identifier,
	parentBlockId flow.Identifier,
) *DerivedBlockData {
	chain.mutex.Lock()
	defer chain.mutex.Unlock()

	currentEntry := chain.unsafeGet(currentBlockId)
	if currentEntry != nil {
		return currentEntry
	}

	var current *DerivedBlockData
	parentEntry, ok := chain.lru.Get(parentBlockId)
	if ok {
		current = parentEntry.NewChildDerivedBlockData()
	} else {
		current = NewEmptyDerivedBlockData(0)
	}

	chain.lru.Add(currentBlockId, current)
	return current
}

func (chain *DerivedChainData) NewDerivedBlockDataForScript(
	currentBlockId flow.Identifier,
) *DerivedBlockData {
	block := chain.Get(currentBlockId)
	if block != nil {
		return block.NewChildDerivedBlockData()
	}

	return NewEmptyDerivedBlockData(0)
}
