package executor

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Failover struct {
	localExecutor         ScriptExecutor
	executionNodeExecutor ScriptExecutor
}

var _ ScriptExecutor = (*Failover)(nil)

func NewFailoverExecutor(localExecutor ScriptExecutor, execNodeExecutor ScriptExecutor) *Failover {
	return &Failover{
		localExecutor:         localExecutor,
		executionNodeExecutor: execNodeExecutor,
	}
}

func (f *Failover) Execute(ctx context.Context, request *ScriptExecutionRequest) ([]byte, time.Duration, error) {
	localResult, localDuration, localErr := f.localExecutor.Execute(ctx, request)

	isInvalidArgument := status.Code(localErr) == codes.InvalidArgument
	isCanceled := status.Code(localErr) == codes.Canceled
	if localErr == nil || isInvalidArgument || isCanceled {
		return localResult, localDuration, localErr
	}

	// Note: scripts that timeout are retried on the execution nodes since ANs may have performance
	// issues for some scripts.
	execResult, execDuration, execErr := f.executionNodeExecutor.Execute(ctx, request)
	return execResult, execDuration, execErr
}
