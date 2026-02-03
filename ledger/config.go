package ledger

import (
	"go.uber.org/atomic"

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
	CheckpointCapacity                   uint
	CheckpointDistance                   uint
	CheckpointsToKeep                    uint
	TriggerCheckpointOnNextSegmentFinish *atomic.Bool
	Metrics                              module.WALMetrics
}

// DefaultCompactorConfig returns default compactor configuration.
func DefaultCompactorConfig(metrics module.WALMetrics) *CompactorConfig {
	return &CompactorConfig{
		CheckpointCapacity:                   DefaultMTrieCacheSize,
		CheckpointDistance:                   DefaultCheckpointDistance,
		CheckpointsToKeep:                    DefaultCheckpointsToKeep,
		TriggerCheckpointOnNextSegmentFinish: atomic.NewBool(false),
		Metrics:                              metrics,
	}
}
