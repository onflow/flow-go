package pipeline

import (
	"context"
	"fmt"
)

// Core defines the interface for pipeline processing steps.
// Each implementation should handle an execution data and implement the three-phase processing:
// download, index, and persist.
type Core interface {
	// Download retrieves all necessary data for processing.
	Download(ctx context.Context) error

	// Index processes the downloaded data and creates in-memory indexes.
	Index(ctx context.Context) error

	// Persist stores the indexed data in permanent storage.
	Persist(ctx context.Context) error

	// Abort performs any necessary cleanup in case of failure.
	Abort(ctx context.Context) error
}

var _ Core = (*CoreImpl)(nil)

type CoreImpl struct{}

func NewCore() *CoreImpl {
	return &CoreImpl{}
}

func (c *CoreImpl) Download(ctx context.Context) error {
	return fmt.Errorf("not implemented")
}

func (c *CoreImpl) Index(ctx context.Context) error {
	return fmt.Errorf("not implemented")
}

func (c *CoreImpl) Persist(ctx context.Context) error {
	return fmt.Errorf("not implemented")
}

func (c *CoreImpl) Abort(ctx context.Context) error {
	return fmt.Errorf("not implemented")
}
