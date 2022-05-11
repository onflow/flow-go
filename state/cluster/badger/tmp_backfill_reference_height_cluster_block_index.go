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
// This procedure is idempotent - it will backfill the index only once and on all
// subsequent calls will be a no-op.
//
// TODO this method should not be included in the master branch.
func BackfillClusterBlockByReferenceHeightIndex(db *badger.DB, log zerolog.Logger, me flow.Identifier, state protocol.State, backfillNum int) error {

	final := state.Final()

	log.Info().Msg("backfilling cluster block reference height index")

	// STEP 1 - check whether the index is populated already, if so exit
	var exists bool
	err := db.View(operation.ClusterBlocksByReferenceHeightIndexExists(&exists))
	if err != nil {
		return fmt.Errorf("could not check index existence")
	}
	if exists {
		log.Info().Msg("cluster block reference height index already exists, exiting...")
		return nil
	}

	// STEP 2 - gather cluster blocks to index from last 2 epochs
	current := final.Epochs().Current()
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

	blocks, err := getFinalizedBlocksForChain(db, cluster.ChainID(), backfillNum)
	if err != nil {
		return fmt.Errorf("could not get curr epoch blocks to index: %w", err)
	}
	log.Info().Msgf("gathered %d/%d blocks to index from current epoch", len(blocks), backfillNum)

	// if there aren't enough blocks in the current epoch, include some from the previous epoch
	if len(blocks) < backfillNum {
		prev := final.Epochs().Previous()
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

		additionalBlocks, err := getFinalizedBlocksForChain(db, cluster.ChainID(), backfillNum-len(blocks))
		if err != nil {
			return fmt.Errorf("could not get prev epoch blocks to index: %w", err)
		}
		blocks = append(blocks, additionalBlocks...)
		log.Info().Msgf("gathered %d/%d blocks to index from previous epoch", len(additionalBlocks), backfillNum)
	}

	// there should always be enough blocks to index in previous two epochs, if we
	// still don't have enough blocks return an error so a human can determine what
	// is going on
	if len(blocks) < backfillNum {
		return fmt.Errorf("could only gather %d/%d blocks from last 2 epochs which is a fatal unexpected condition", len(blocks), backfillNum)
	}

	// check for dupes
	ids := make(map[flow.Identifier]struct{})
	for _, block := range blocks {
		ids[block.clusterBlockID] = struct{}{}
	}
	fmt.Println("number of unique blocks to index: ", len(ids))

	// STEP 3 - batch index the gathered cluster blocks
	log.Info().Msgf("about to index %d most recent cluster blocks", len(blocks))

	batch := db.NewWriteBatch()
	for i, block := range blocks {
		if i%(backfillNum/10) == 0 {
			log.Info().Msgf("indexed %d/%d cluster blocks", i, len(blocks))
		}
		err = operation.BatchIndexClusterBlockByReferenceHeight(block.referenceHeight, block.clusterBlockID)(batch)
		if err != nil {
			return fmt.Errorf("could not batch insert cluster block (ref_height=%d, id=%x): %w", block.referenceHeight, block.clusterBlockID, err)
		}
	}
	err = batch.Flush()
	if err != nil {
		return fmt.Errorf("could not flush batch indexing cluster blocks: %w", err)
	}

	log.Info().Msg("finished backfilling cluster block reference height index")

	return nil
}

// TODO this method should not be included in the master branch.
func getFinalizedBlocksForChain(db *badger.DB, chainID flow.ChainID, limit int) ([]indexableBlock, error) {
	fmt.Println("chain ID: ", chainID.String())
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
			return fmt.Errorf("could not get finalized block ID: %w", err)
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
