package environment

import (
	"time"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/flow-go/module/trace"
)

// MetricsReporter captures and reports metrics to back to the execution
// environment it is a setup passed to the context.
type MetricsReporter interface {
	RuntimeTransactionParsed(time.Duration)
	RuntimeTransactionChecked(time.Duration)
	RuntimeTransactionInterpreted(time.Duration)
	RuntimeSetNumberOfAccounts(count uint64)
}

// NoopMetricsReporter is a MetricReporter that does nothing.
type NoopMetricsReporter struct{}

// RuntimeTransactionParsed is a noop
func (NoopMetricsReporter) RuntimeTransactionParsed(time.Duration) {}

// RuntimeTransactionChecked is a noop
func (NoopMetricsReporter) RuntimeTransactionChecked(time.Duration) {}

// RuntimeTransactionInterpreted is a noop
func (NoopMetricsReporter) RuntimeTransactionInterpreted(time.Duration) {}

// RuntimeSetNumberOfAccounts is a noop
func (NoopMetricsReporter) RuntimeSetNumberOfAccounts(count uint64) {}

type ProgramLogger struct {
	tracer *Tracer

	logger zerolog.Logger

	cadenceLoggingEnabled bool
	logs                  []string

	reporter MetricsReporter
}

func NewProgramLogger(
	tracer *Tracer,
	logger zerolog.Logger,
	reporter MetricsReporter,
	cadenceLoggingEnabled bool,
) *ProgramLogger {
	return &ProgramLogger{
		tracer:                tracer,
		logger:                logger,
		cadenceLoggingEnabled: cadenceLoggingEnabled,
		logs:                  nil,
		reporter:              reporter,
	}
}

func (logger *ProgramLogger) Logger() *zerolog.Logger {
	return &logger.logger
}

func (logger *ProgramLogger) ImplementationDebugLog(message string) error {
	logger.logger.Debug().Msgf("Cadence: %s", message)
	return nil
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
