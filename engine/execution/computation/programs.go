package computation

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru"

	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/model/flow"
)

const DefaultProgramsCacheSize = 1000

type ProgramsCache struct {
	cache *lru.Cache
}

func NewProgramsCache(size uint) (*ProgramsCache, error) {
	cache, err := lru.New(int(size))
	if err != nil {
		return nil, fmt.Errorf("cannot crete LRU cache: %w", err)
	}
	return &ProgramsCache{
		cache: cache,
	}, nil
}

func (pc *ProgramsCache) Get(blockID flow.Identifier) programs.Programs {
	get, ok := pc.cache.Get(blockID)
	if !ok {
		return nil
	}
	return get.(programs.Programs)
}

func (pc *ProgramsCache) Set(blockId flow.Identifier, programs programs.Programs) {
	pc.cache.Add(blockId, programs)
}
