package optimistic_sync

import "github.com/onflow/flow-go/model/flow"

// CoreFactory is a factory object for creating new Core instances.
type CoreFactory interface {
	NewCore(result *flow.ExecutionResult) Core
}

// PipelineFactory is a factory object for creating new Pipeline instances.
type PipelineFactory interface {
	NewPipeline(result *flow.ExecutionResult) Pipeline
}
