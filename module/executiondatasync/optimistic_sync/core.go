package optimistic_sync

import (
	"context"
	"fmt"
)

// Core defines the interface for pipeline processing steps.
// Each implementation should handle an execution data and implement the three-phase processing:
// download, index, and persist.
type Core interface {
	// Download retrieves all necessary data for processing.
	// Expected errors:
	// - context.Canceled: if the provided context was canceled before completion
	//
	// All other errors are unexpected and may indicate a bug or inconsistent state
	Download(ctx context.Context) error

	// Index processes the downloaded data and creates in-memory indexes.
	//
	// No errors are expected during normal operations
	Index() error

	// Persist stores the indexed data in permanent storage.
	//
	// No errors are expected during normal operations
	Persist() error

	// Abandon indicates that the protocol has abandoned this state. Hence processing will be aborted
	// and any data dropped.
	//
	// No errors are expected during normal operations
	Abandon() error
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

func (c *CoreImpl) Abandon(ctx context.Context) error {
	return fmt.Errorf("not implemented")
}
