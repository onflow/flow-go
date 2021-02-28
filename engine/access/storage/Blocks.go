package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type Blocks struct {
	storage.Blocks
}

func (blocks *Blocks) Store(block *flow.Block) error {
	err := blocks.Blocks.Store(block)
	return panicOnError(err)
}

func panicOnError(err error) error {
	if err != nil {
		panic(err)
	}
	return err
}
