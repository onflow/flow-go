package verification

import (
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// ChunkDataPackRequest is an internal data structure in fetcher engine that is passed between the engine
// and requester module. It conveys required information for requesting a chunk data pack.
type ChunkDataPackRequest struct {
	chunks.Locator
	ChunkDataPackRequestInfo
}

type ChunkDataPackRequestInfo struct {
	ChunkID   flow.Identifier
	Height    uint64              // block height of execution result of the chunk, used to drop chunk requests of sealed heights.
	Agrees    flow.IdentifierList // execution node ids that generated the result of chunk.
	Disagrees flow.IdentifierList // execution node ids that generated a conflicting result with result of chunk.
	Targets   flow.IdentityList   // list of all execution nodes identity at the block height of this chunk (including non-responders).
}

//func (c ChunkDataPackRequest) ID() flow.Identifier {
//	return c.Locator.ID()
//}
//
//func (c ChunkDataPackRequest) Checksum() flow.Identifier {
//	return c.Locator.ID()
//}

// SampleTargets returns identifier of execution nodes that can be asked for the chunk data pack, based on
// the agreeing and disagreeing execution nodes of the chunk data pack request.
func (c ChunkDataPackRequestInfo) SampleTargets(count int) flow.IdentifierList {
	// if there are enough receipts produced the same result (agrees), we sample from them.
	if len(c.Agrees) >= count {
		return c.Targets.Filter(filter.HasNodeID(c.Agrees...)).Sample(uint(count)).NodeIDs()
	}

	// since there is at least one agree, then usually, we just need `count - 1` extra nodes as backup.
	// We pick these extra nodes randomly from the rest nodes who we haven't received its receipt.
	// In the case where all other execution nodes has produced different results, then we will only
	// fetch from the one produced the same result (the only agree)
	need := uint(count - len(c.Agrees))

	nonResponders := c.Targets.Filter(filter.Not(filter.HasNodeID(c.Disagrees...))).Sample(need).NodeIDs()
	return append(c.Agrees, nonResponders...)
}

type ChunkDataPackRequestInfoList []*ChunkDataPackRequestInfo
type ChunkDataPackRequestList []*ChunkDataPackRequest

// ContainsChunkID returns true if list contains a request for chunk ID.
func (c ChunkDataPackRequestList) ContainsChunkID(chunkID flow.Identifier) bool {
	for _, request := range c {
		if request.ChunkID == chunkID {
			return true
		}
	}

	return false
}

// ContainsLocator returns true if list contains a request for the given resultID and chunkIndex.
func (c ChunkDataPackRequestList) ContainsLocator(resultID flow.Identifier, chunkIndex uint64) bool {
	for _, request := range c {
		if request.ResultID == resultID && request.Index == chunkIndex {
			return true
		}
	}

	return false
}

// UniqueRequestInfo extracts and returns request info based on chunk IDs. Note that a ChunkDataPackRequestList
// may have duplicate requests for the same chunk ID that belongs to distinct execution results.
func (c ChunkDataPackRequestList) UniqueRequestInfo() ChunkDataPackRequestInfoList {
	added := make(map[flow.Identifier]struct{})
	requestInfoList := ChunkDataPackRequestInfoList{}

	for _, request := range c {
		if _, ok := added[request.ChunkID]; !ok {
			info := request.ChunkDataPackRequestInfo
			requestInfoList = append(requestInfoList, &info)
			added[request.ChunkID] = struct{}{}
		}
	}

	return requestInfoList
}
