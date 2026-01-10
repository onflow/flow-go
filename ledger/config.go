package ledger

import (
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module"
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
		CheckpointCapacity:                   100,
		CheckpointDistance:                   100,
		CheckpointsToKeep:                    3,
		TriggerCheckpointOnNextSegmentFinish: atomic.NewBool(false),
		Metrics:                              metrics,
	}
}
