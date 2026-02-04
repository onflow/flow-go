package ledger

import (
	"github.com/onflow/flow-go/module"
)

// Default values for ledger configuration.
const (
	DefaultMTrieCacheSize     = 500
	DefaultCheckpointDistance = 20
	DefaultCheckpointsToKeep  = 5
)

// CompactorConfig holds configuration for ledger compaction.
type CompactorConfig struct {
	CheckpointCapacity uint
	CheckpointDistance uint
	CheckpointsToKeep  uint
	Metrics            module.WALMetrics
}

// DefaultCompactorConfig returns default compactor configuration.
func DefaultCompactorConfig(metrics module.WALMetrics) *CompactorConfig {
	return &CompactorConfig{
		CheckpointCapacity: DefaultMTrieCacheSize,
		CheckpointDistance: DefaultCheckpointDistance,
		CheckpointsToKeep:  DefaultCheckpointsToKeep,
		Metrics:            metrics,
	}
}
