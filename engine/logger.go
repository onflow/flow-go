package engine

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/logging"
)

type  FlowLogger struct {
	*zerolog.Logger
}

func NewFlowLogger(loger *zerolog.Logger) *FlowLogger {
	return &FlowLogger{loger}
}


type FlowEvent struct {
	*zerolog.Event
}

func (l *FlowLogger) Debug() *FlowEvent {
	return &FlowEvent{l.Logger.Debug()}
}

func (f *FlowEvent) Entity(key string, entity flow.Entity) *FlowEvent {
	return &FlowEvent{f.Hex(key, logging.Entity(entity))}
}
