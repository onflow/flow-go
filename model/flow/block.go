// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"time"

	"github.com/dapperlabs/flow-go/crypto"
)

type Block struct {
	Header
	NewIdentities        IdentityList
	CollectionGuarantees []*CollectionGuarantee
}

func Genesis(ids IdentityList) *Block {

	header := Header{
		Number:    0,
		Timestamp: time.Unix(1575244800, 0),
		Parent:    crypto.ZeroHash,
	}

	genesis := Block{
		Header:        header,
		NewIdentities: ids,
	}

	genesis.Header.Payload = genesis.Payload()
	return &genesis
}

func (b Block) Payload() crypto.Hash {
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)
	for _, id := range b.NewIdentities {
		hasher.Add(id.Encode())
	}
	for _, guarantee := range b.CollectionGuarantees {
		hasher.Add(guarantee.Hash)
	}
	return hasher.SumHash()
}
