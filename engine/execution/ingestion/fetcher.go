package ingestion

import "github.com/onflow/flow-go/model/flow"

// CollectionFetcher abstracts the details of how to fetch collection
type CollectionFetcher interface {
	// FetchCollection decides which collection nodes to fetch the collection from
	// No error is expected during normal operation
	FetchCollection(blockID flow.Identifier, height uint64, guarantee *flow.CollectionGuarantee) error

	// Force forces the requests to be sent immediately
	Force()
}
