package fetcher_test

import (
	"github.com/onflow/flow-go/engine/execution/ingestion"
	"github.com/onflow/flow-go/engine/execution/ingestion/fetcher"
)

var _ ingestion.CollectionFetcher = (*fetcher.CollectionFetcher)(nil)
