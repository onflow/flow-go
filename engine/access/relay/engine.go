package relay

import (
	"github.com/onflow/flow-go/engine"
	"github.com/rs/zerolog"
)

type Config struct {
}

type Engine struct {
	unit   *engine.Unit   // used to manage concurrency & shutdown
	log    zerolog.Logger // used to log relevant actions with context
	config Config
}

func New() *Engine {
	return &Engine{}
}
