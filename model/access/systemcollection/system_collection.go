package systemcollection

import (
	"fmt"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

const (
	Version0 access.Version = 0
	Version1 access.Version = 1
)

var ChainHeightVersions = map[flow.ChainID]access.HeightVersionMapper{
	flow.Testnet: access.NewStaticHeightVersionMapper(map[uint64]access.Version{
		0:         Version0,
		290050888: Version1,
	}),
	flow.Mainnet: access.NewStaticHeightVersionMapper(map[uint64]access.Version{
		0:         Version0,
		133084444: Version1,
	}),
}

// versionBuilder is a map of all versions of the system collection.
var versionBuilder = map[access.Version]access.SystemCollectionBuilder{
	Version0:             &builderV0{},
	Version1:             &builderV1{},
	access.VersionLatest: &builderV1{},
}

// Default returns the default versioned system collection builder for the provided chain.
// This is the set of all available version builders for the chain.
func Default(chainID flow.ChainID) *access.Versioned[access.SystemCollectionBuilder] {
	versionMapper, ok := ChainHeightVersions[chainID]
	if !ok {
		versionMapper = access.NewStaticHeightVersionMapper(access.LatestBoundary)
	}
	return access.NewVersioned(versionBuilder, versionMapper)
}

// Versioned is a collection of all versions of the system collections.
//
// This is useful when we want to obtain a system collection by ID but we don't have the block height,
// so we check all versions of system collection if it matches by ID.
type Versioned struct {
	chain        flow.Chain
	transactions map[flow.Identifier]*flow.TransactionBody
	versioned    *access.Versioned[access.SystemCollectionBuilder]
}

func NewVersioned(chain flow.Chain, versioned *access.Versioned[access.SystemCollectionBuilder]) (*Versioned, error) {
	// build and cache all system transactions from all versions for efficient lookup by ID.
	// this is commonly used within the transactions API endpoints.
	transactions := make(map[flow.Identifier]*flow.TransactionBody)
	for _, builder := range versioned.All() {
		collection, err := builder.SystemCollection(chain, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to construct system collection: %w", err)
		}

		for _, tx := range collection.Transactions {
			transactions[tx.ID()] = tx
		}
	}

	return &Versioned{
		chain:        chain,
		transactions: transactions,
		versioned:    versioned,
	}, nil
}

// SearchAll searches for a transaction by ID in all versions of the system collections.
// Returns true if the transaction was found in any of the versions, false otherwise.
//
// This is useful when looking up transactions by ID where the block is not known.
func (s *Versioned) SearchAll(id flow.Identifier) (*flow.TransactionBody, bool) {
	tx, ok := s.transactions[id]
	return tx, ok
}

// ByHeight returns the system collection builder for the given height.
func (s *Versioned) ByHeight(height uint64) access.SystemCollectionBuilder {
	return s.versioned.ByHeight(height)
}
