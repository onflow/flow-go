package ingestion

import "github.com/onflow/flow-go/model/flow"

type CollectionFetcher interface {
	FetchCollection(blockID flow.Identifier, height uint64, guarantee *flow.CollectionGuarantee) error
	Force()
}
