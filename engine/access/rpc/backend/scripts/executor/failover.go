package executor

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TODO(Uliana): add godoc to whole file
type FailoverScriptExecutor struct {
	localExecutor         ScriptExecutor
	executionNodeExecutor ScriptExecutor
}

var _ ScriptExecutor = (*FailoverScriptExecutor)(nil)

func NewFailoverScriptExecutor(localExecutor ScriptExecutor, execNodeExecutor ScriptExecutor) *FailoverScriptExecutor {
	return &FailoverScriptExecutor{
		localExecutor:         localExecutor,
		executionNodeExecutor: execNodeExecutor,
	}
}

func (f *FailoverScriptExecutor) Execute(ctx context.Context, request *Request) ([]byte, time.Duration, error) {
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
