package fvm

import (
	"github.com/onflow/flow-go/module/trace"
)

type ProgramLogger struct {
	ctx *EnvContext

	logs []string
}

func NewProgramLogger(ctx *EnvContext) *ProgramLogger {
	return &ProgramLogger{
		ctx:  ctx,
		logs: nil,
	}
}

func (logger *ProgramLogger) ProgramLog(message string) error {
	defer logger.ctx.StartExtensiveTracingSpanFromRoot(trace.FVMEnvProgramLog).End()

	if logger.ctx.CadenceLoggingEnabled {
		logger.logs = append(logger.logs, message)
	}
	return nil
}

func (logger *ProgramLogger) Logs() []string {
	return logger.logs
}
