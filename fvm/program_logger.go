package fvm

import (
	"time"

	"github.com/onflow/cadence/runtime/common"
	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/module/trace"
)

type ProgramLogger struct {
	tracer *Tracer

	cadenceLoggingEnabled bool
	logs                  []string

	reporter handler.MetricsReporter
}

func NewProgramLogger(
	tracer *Tracer,
	reporter handler.MetricsReporter,
	cadenceLoggingEnabled bool,
) *ProgramLogger {
	return &ProgramLogger{
		tracer:                tracer,
		cadenceLoggingEnabled: cadenceLoggingEnabled,
		logs:                  nil,
		reporter:              reporter,
	}
}

func (logger *ProgramLogger) ProgramLog(message string) error {
	defer logger.tracer.StartExtensiveTracingSpanFromRoot(trace.FVMEnvProgramLog).End()

	if logger.cadenceLoggingEnabled {
		logger.logs = append(logger.logs, message)
	}
	return nil
}

func (logger *ProgramLogger) Logs() []string {
	return logger.logs
}

func (logger *ProgramLogger) RecordTrace(operation string, location common.Location, duration time.Duration, attrs []attribute.KeyValue) {
	if location != nil {
		attrs = append(attrs, attribute.String("location", location.String()))
	}
	logger.tracer.RecordSpanFromRoot(
		trace.FVMCadenceTrace.Child(operation),
		duration,
		attrs)
}

// ProgramParsed captures time spent on parsing a code at specific location
func (logger *ProgramLogger) ProgramParsed(location common.Location, duration time.Duration) {
	logger.RecordTrace("parseProgram", location, duration, nil)

	// These checks prevent re-reporting durations, the metrics collection is
	// a bit counter-intuitive:
	// The three functions (parsing, checking, interpretation) are not called
	// in sequence, but in some cases as part of each other. We basically only
	// measure the durations reported for the entry-point (the transaction),
	// and not for child locations, because they might be already part of the
	// duration for the entry-point.
	if location == nil {
		return
	}
	if _, ok := location.(common.TransactionLocation); ok {
		logger.reporter.RuntimeTransactionParsed(duration)
	}
}

// ProgramChecked captures time spent on checking a code at specific location
func (logger *ProgramLogger) ProgramChecked(location common.Location, duration time.Duration) {
	logger.RecordTrace("checkProgram", location, duration, nil)

	// see the comment for ProgramParsed
	if location == nil {
		return
	}
	if _, ok := location.(common.TransactionLocation); ok {
		logger.reporter.RuntimeTransactionChecked(duration)
	}
}

// ProgramInterpreted captures time spent on interpreting a code at specific location
func (logger *ProgramLogger) ProgramInterpreted(location common.Location, duration time.Duration) {
	logger.RecordTrace("interpretProgram", location, duration, nil)

	// see the comment for ProgramInterpreted
	if location == nil {
		return
	}
	if _, ok := location.(common.TransactionLocation); ok {
		logger.reporter.RuntimeTransactionInterpreted(duration)
	}
}

// ValueEncoded accumulates time spend on runtime value encoding
func (logger *ProgramLogger) ValueEncoded(duration time.Duration) {
	logger.RecordTrace("encodeValue", nil, duration, nil)
}

// ValueDecoded accumulates time spend on runtime value decoding
func (logger *ProgramLogger) ValueDecoded(duration time.Duration) {
	logger.RecordTrace("decodeValue", nil, duration, nil)
}
