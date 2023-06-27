package uploader

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
)

// Manager encapsulates the logic for uploading computation results to cloud
// storage.
type Manager struct {
	enabled   bool
	tracer    module.Tracer
	uploaders []Uploader
	mu        sync.RWMutex
}

// NewManager creates a new uploader manager
func NewManager(tracer module.Tracer) *Manager {
	return &Manager{
		tracer: tracer,
	}
}

// AddUploader adds an uploader to the manager
func (m *Manager) AddUploader(uploader Uploader) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.enabled = true
	m.uploaders = append(m.uploaders, uploader)
}

// SetEnabled enables or disables the manager
func (m *Manager) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.enabled = enabled
}

// Enabled returns whether the manager is enabled
func (m *Manager) Enabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.enabled
}

// Upload uploads the given computation result with all uploaders
// Any errors returned by the uploaders may be considered benign
func (m *Manager) Upload(
	ctx context.Context,
	result *execution.ComputationResult,
) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled {
		return nil
	}

	var group errgroup.Group

	for _, uploader := range m.uploaders {
		uploader := uploader

		group.Go(func() error {
			span, _ := m.tracer.StartSpanFromContext(ctx, trace.EXEUploadCollections)
			defer span.End()

			return uploader.Upload(result)
		})
	}

	return group.Wait()
}

// RetryUploads retries uploads for all uploaders that implement RetryableUploaderWrapper
// Any errors returned by the uploaders may be considered benign
func (m *Manager) RetryUploads() (err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled {
		return nil
	}

	for _, u := range m.uploaders {
		switch retryableUploaderWraper := u.(type) {
		case RetryableUploaderWrapper:
			err = retryableUploaderWraper.RetryUpload()
		}
	}
	return err
}
