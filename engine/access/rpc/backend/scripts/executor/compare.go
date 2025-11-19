package executor

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/module"
)

type ComparingScriptExecutor struct {
	log     zerolog.Logger
	metrics module.BackendScriptsMetrics

	localExecutor         ScriptExecutor
	executionNodeExecutor ScriptExecutor

	scriptCache *LoggedScriptCache
}

var _ ScriptExecutor = (*ComparingScriptExecutor)(nil)

func NewComparingScriptExecutor(
	log zerolog.Logger,
	metrics module.BackendScriptsMetrics,
	scriptCache *LoggedScriptCache,
	localExecutor ScriptExecutor,
	execNodeExecutor ScriptExecutor,
) *ComparingScriptExecutor {
	return &ComparingScriptExecutor{
		log:                   log.With().Str("script_executor", "comparing").Logger(),
		metrics:               metrics,
		scriptCache:           scriptCache,
		localExecutor:         localExecutor,
		executionNodeExecutor: execNodeExecutor,
	}
}

func (c *ComparingScriptExecutor) Execute(ctx context.Context, request *Request) ([]byte, time.Duration, error) {
	execResult, execDuration, execErr := c.executionNodeExecutor.Execute(ctx, request)

	// we can only compare the results if there were either no errors or a cadence error
	// since we cannot distinguish the EN error as caused by the block being pruned or some other reason,
	// which may produce a valid RN output but an error for the EN
	isInvalidArgument := status.Code(execErr) == codes.InvalidArgument
	if execErr != nil && !isInvalidArgument {
		return nil, 0, execErr
	}

	localResult, localDuration, localErr := c.localExecutor.Execute(ctx, request)

	resultComparer := newScriptResultComparison(c.log, c.metrics, c.scriptCache.shouldLogScript, request)
	_ = resultComparer.compare(
		newScriptResult(execResult, execDuration, execErr),
		newScriptResult(localResult, localDuration, localErr),
	)

	return execResult, execDuration, execErr
}
