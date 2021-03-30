package chunkrequester

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/engine/verification/match"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/logging"
)

type ChunkDataPackRequester struct {
	log           zerolog.Logger
	handler       fetcher.ChunkDataPackHandler // contains callbacks for handling received chunk data packs.
	retryInterval time.Duration                // determines time in milliseconds for retrying chunk data requests.
	pendingChunks *match.Chunks                // used to store all the pending chunks that assigned to this node
	state         protocol.State               // used to check the last sealed height
}

func New(log zerolog.Logger,
	state protocol.State,
	retryInterval time.Duration,
	handler fetcher.ChunkDataPackHandler) *ChunkDataPackRequester {
	return &ChunkDataPackRequester{
		log:           log.With().Str("engine", "requester").Logger(),
		retryInterval: retryInterval,
		handler:       handler,
		state:         state,
	}
}

func (c *ChunkDataPackRequester) Request(chunkID flow.Identifier, blockID flow.Identifier, targetIDs flow.IdentifierList) error {
	return nil
}

// onTimer should run periodically, it goes through all pending chunks, and requests their chunk data pack.
// It also retries the chunk data request if the data hasn't been received for a while.
func (c *ChunkDataPackRequester) onTimer() {
	allChunks := c.pendingChunks.All()

	c.log.Debug().Int("total", len(allChunks)).Msg("start processing all pending pendingChunks")

	sealed, err := c.state.Sealed().Head()
	if err != nil {
		c.log.Error().Err(err).Msg("could not get last sealed block")
		return
	}

	allExecutors, err := c.state.Final().Identities(filter.HasRole(flow.RoleExecution))
	if err != nil {
		c.log.Error().Err(err).Msg("could not get executors")
		return
	}

	for _, chunk := range allChunks {
		chunkID := chunk.ID()

		log := c.log.With().
			Hex("chunk_id", logging.ID(chunkID)).
			Hex("result_id", logging.ID(chunk.ExecutionResultID)).
			Hex("block_id", logging.ID(chunk.Chunk.ChunkBody.BlockID)).
			Uint64("height", chunk.Height).
			Logger()

		// if block has been sealed, then we can finish
		isSealed := chunk.Height <= sealed.Height

		if isSealed {
			removed := c.pendingChunks.Rem(chunkID)
			log.Info().Bool("removed", removed).Msg("chunk has been sealed, no longer needed")
			continue
		}

		err := c.requestChunkDataPack(chunk, allExecutors)
		if err != nil {
			log.Warn().Err(err).Msg("could not request chunk data pack")
			continue
		}

		log.Info().Msg("chunk data requested")
	}
}
