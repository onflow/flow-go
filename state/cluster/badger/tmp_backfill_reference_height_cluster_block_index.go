package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	model "github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
)

type indexableBlock struct {
	clusterBlockID  flow.Identifier
	referenceHeight uint64
}

// BackfillClusterBlockByReferenceHeightIndex populates the reference height to cluster block
// index for all blocks in this node's cluster for the previous 2 epochs. This is intended
// for use when initially deploying transaction deduplication mid-spork.
//
// This method should not be included in the master branch.
func BackfillClusterBlockByReferenceHeightIndex(db *badger.DB, log zerolog.Logger, me flow.Identifier, state protocol.State) error {

	const BACKFILL_BLOCKS = 10_000

	epochs := state.Final().Epochs()

	current := epochs.Current()
	clustering, err := current.Clustering()
	if err != nil {
		return fmt.Errorf("could not get clustering: %w", err)
	}
	_, idx, ok := clustering.ByNodeID(me)
	if !ok {
		return fmt.Errorf("could not find self in clustering")
	}
	cluster, err := current.Cluster(idx)
	if err != nil {
		return fmt.Errorf("could not get cluster: %w", err)
	}

	blocks, err := getFinalizedBlocksForChain(db, cluster.ChainID(), BACKFILL_BLOCKS)
	if err != nil {
		return fmt.Errorf("could not get curr epoch blocks to index: %w", err)
	}

	// if there aren't enough blocks in the current epoch, include some from the previous epoch
	if len(blocks) < BACKFILL_BLOCKS {
		prev := epochs.Previous()
		clustering, err = prev.Clustering()
		if err != nil {
			return fmt.Errorf("could not get clustering: %w", err)
		}
		_, idx, ok = clustering.ByNodeID(me)
		if !ok {
			return fmt.Errorf("could not find self in clustering")
		}
		cluster, err = prev.Cluster(idx)
		if err != nil {
			return fmt.Errorf("could not get cluster: %w", err)
		}

		additionalBlocks, err := getFinalizedBlocksForChain(db, cluster.ChainID(), BACKFILL_BLOCKS-len(blocks))
		if err != nil {
			return fmt.Errorf("could not get prev epoch blocks to index: %w", err)
		}
		blocks = append(blocks, additionalBlocks...)
	}

}

func getFinalizedBlocksForChain(db *badger.DB, chainID flow.ChainID, limit int) ([]indexableBlock, error) {
	var blocks []indexableBlock
	err := db.View(func(tx *badger.Txn) error {
		var finalHeight uint64
		err := operation.RetrieveClusterFinalizedHeight(chainID, &finalHeight)(tx)
		if err != nil {
			return fmt.Errorf("could not get finalized height: %w", err)
		}
		var finalID flow.Identifier
		err = operation.LookupClusterBlockHeight(chainID, finalHeight, &finalID)(tx)
		if err != nil {
			return fmt.Errorf("could not finalized block ID: %w", err)
		}
		var final model.Block
		err = procedure.RetrieveClusterBlock(finalID, &final)(tx)
		if err != nil {
			return fmt.Errorf("could not get finalized block: %w", err)
		}

		next := final
		for next.Header.Height > 0 && len(blocks) < limit {
			var refBlock flow.Header
			err = operation.RetrieveHeader(next.Payload.ReferenceBlockID, &refBlock)(tx)
			if err != nil {
				return fmt.Errorf("could not get reference block: %w", err)
			}
			blocks = append(blocks, indexableBlock{
				clusterBlockID:  next.ID(),
				referenceHeight: refBlock.Height,
			})

			err = procedure.RetrieveClusterBlock(next.Header.ParentID, &next)(tx)
			if err != nil {
				return fmt.Errorf("could not get block: %w", err)
			}
		}
		return nil
	})
	return blocks, err
}
