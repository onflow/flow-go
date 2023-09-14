package loader_test

import (
	"github.com/onflow/flow-go/engine/execution/ingestion"
	"github.com/onflow/flow-go/engine/execution/ingestion/loader"
)

var _ ingestion.BlockLoader = (*loader.Loader)(nil)
