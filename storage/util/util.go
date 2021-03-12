package util

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type onBlockTraverse = func(header *flow.Header) error
type onShouldContinue = func(header *flow.Header) bool

func TraverseBlocksBackwards(headers storage.Headers, startBlockID flow.Identifier,
	shouldContinue onShouldContinue, traverse onBlockTraverse) error {
	ancestorID := startBlockID
	for {

		ancestor, err := headers.ByBlockID(startBlockID)
		if err != nil {
			return fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
		}
		if !shouldContinue(ancestor) {
			break
		}

		traverse(ancestor)

		ancestorID = ancestor.ParentID
	}
	return nil
}
