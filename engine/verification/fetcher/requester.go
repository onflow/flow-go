package fetcher

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// ChunkDataPackRequester encapsulates the logic of requesting a chunk data pack from an execution node.
type ChunkDataPackRequester interface {
	// Request makes the request of chunk data pack for the specified chunk ID from the target IDs.
	Request(request *ChunkDataPackRequest)
}

// ChunkDataPackHandler encapsulates the logic of handling a requested chunk data pack upon its arrival.
type ChunkDataPackHandler interface {
	// HandleChunkDataPack is called by the ChunkDataPackRequester anytime a new requested chunk arrives.
	// It contains the logic of handling the chunk data pack.
	HandleChunkDataPack(originID flow.Identifier, chunkDataPack *flow.ChunkDataPack, collection *flow.Collection)

	// NotifyChunkDataPackSealed is called by the ChunkDataPackRequester to notify the ChunkDataPackHandler that the chunk ID has been sealed and
	// hence the requester will no longer request it.
	//
	// When the requester calls this callback method, it will never returns a chunk data pack for this chunk ID to the handler (i.e.,
	// through HandleChunkDataPack).
	NotifyChunkDataPackSealed(chunkID flow.Identifier)
}

// ChunkDataPackRequest is an internal data structure in fetcher engine that is passed between the engine
// and requester module. It conveys required information for requesting a chunk data pack.
type ChunkDataPackRequest struct {
	ChunkID   flow.Identifier
	Height    uint64            // block height of execution result of the chunk, used to drop chunk requests of sealed heights.
	Agrees    []flow.Identifier // execution node ids that generated the result of chunk.
	Disagrees []flow.Identifier // execution node ids that generated a conflicting result with result of chunk.
}

// SampleTargets returns identifier of execution nodes that can be asked for the chunk data pack, based on
// the agree and disagree execution nodes of the chunk data pack request.
func (c ChunkDataPackRequest) SampleTargets(all flow.IdentityList, count int) flow.IdentifierList {
	// if there are enough receipts produced the same result (agrees), we sample from them.
	if len(c.Agrees) >= count {
		return all.Filter(filter.HasNodeID(c.Agrees...)).Sample(uint(count)).NodeIDs()
	}

	// since there is at least one agree, then usually, we just need `count - 1` extra nodes as backup.
	// We pick these extra nodes randomly from the rest nodes who we haven't received its receipt.
	// In the case where all other execution nodes has produced different results, then we will only
	// fetch from the one produced the same result (the only agree)
	need := uint(count - len(c.Agrees))

	nonResponders := all.Filter(filter.Not(filter.HasNodeID(c.Disagrees...))).Sample(need).NodeIDs()
	return append(c.Agrees, nonResponders...)
}
