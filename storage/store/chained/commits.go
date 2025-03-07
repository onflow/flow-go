package chained

import (
	"errors"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type ChainedCommits struct {
	first  storage.CommitsReader
	second storage.CommitsReader
}

var _ storage.CommitsReader = (*ChainedCommits)(nil)

// NewCommits returns a new ChainedCommits commits store, which will handle reads. Any writes query
// will return err
// for reads, it first query first database, then second database, this is useful when migrating
// data from badger to pebble
func NewCommits(first storage.CommitsReader, second storage.CommitsReader) *ChainedCommits {
	return &ChainedCommits{
		first:  first,
		second: second,
	}
}

func (c *ChainedCommits) ByBlockID(blockID flow.Identifier) (flow.StateCommitment, error) {
	result, err := c.first.ByBlockID(blockID)
	if err == nil {
		return result, nil
	}

	if errors.Is(err, storage.ErrNotFound) {
		return c.second.ByBlockID(blockID)
	}

	return flow.StateCommitment{}, err
}
