package bft

import "github.com/dapperlabs/bamboo-node/internal/roles/consensus/mempool"

type Block struct {
	ID uint
	Hash []byte
	Payload mempool.BlockPayload
}


type FinalizedBlock struct {
	Block
	Signatures AggregatedSignature
}

type AggregatedSignature interface {

}

