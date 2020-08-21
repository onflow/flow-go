package ingestion

import (
	"errors"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/storage"
)

// requestMissingCollections requests missing collections for all blocks in the local db storage
func requestMissingCollections(finalizedHeight uint64,
	blocks storage.Blocks,
	collections storage.Collections,
	request module.Requester,
	log zerolog.Logger) {

	log.Info().Uint64("height", finalizedHeight).Msg("starting collection catchup")
	missingCollCount := uint64(0)

	// iterator through the complete chain (but only request collections for blocks we have)
	for i := finalizedHeight; i >= 0; i-- {

		// for all the blocks in the local db, request missing collections
		blk, err := blocks.ByHeight(i)
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				log.Error().Err(err).Uint64("height", i).Msg("failed to retrieve block")
			}
			// if block not found locally, continue
			continue
		}

		for _, guarantee := range blk.Payload.Guarantees {
			collId := guarantee.CollectionID
			_, err = collections.LightByID(collId)

			// if collection found, continue
			if err != nil {
				continue
			}

			if errors.Is(err, storage.ErrNotFound) {

				// requesting the collection from the collection node
				log.Debug().Str("collection_id", collId.String()).Msg("requesting missing collection")
				request.EntityByID(guarantee.ID(), filter.HasNodeID(guarantee.SignerIDs...))
				missingCollCount++
			} else {
				log.Error().Err(err).Str("collection_id", collId.String()).Msg("failed to retrieve collection from storage")
			}
		}
	}

	// the collection catchup needs to happen ASAP when the node starts up. Hence, force the requester to dispatch all request.
	if missingCollCount > 0 {
		request.Force()
	}

	log.Info().Uint64("total_missing_collections", missingCollCount).Msg("collection catchup done")
}
