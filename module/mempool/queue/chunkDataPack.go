package queue

import "github.com/onflow/flow-go/module/mempool"

type ChunkDataPackRequestQueue struct {
}

func (c ChunkDataPackRequestQueue) Push(chunkId flow.Identifier, requesterId flow.Identifier) bool {
	//TODO implement me
	panic("implement me")
}

func (c ChunkDataPackRequestQueue) Pop() (flow.Identifier, flow.Identifier, bool) {
	//TODO implement me
	panic("implement me")
}

func (c ChunkDataPackRequestQueue) Size() uint {
	//TODO implement me
	panic("implement me")
}

var _ mempool.ChunkDataPackRequestQueue = &ChunkDataPackRequestQueue{}
