package block_iterator

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/block_iterator/height_based"
	"github.com/onflow/flow-go/storage"
)

func NewHeightIteratorFactory(
	ctx context.Context,
	headers storage.Headers,
	progress storage.ConsumerProgress,
	getRoot func() (*flow.Header, error),
	latest func() (*flow.Header, error),
) (*module.IteratorFactory, error) {

	initializer := height_based.NewInitializer(progress, getRoot)
	creator := height_based.NewHeightBasedIteratorCreator(ctx, headers)
	jobCreator := height_based.NewHeightIteratorJobCreator(latest)

	return module.NewIteratorFactory(initializer, creator, jobCreator)
}
