package liveness

import (
	"github.com/dapperlabs/flow-go/engine"
)

type Server struct {
	unit      *engine.Unit
	collector *CheckCollector
}

func NewServer(cc *CheckCollector) *Server {
	return &Server{
		unit:      engine.NewUnit(),
		collector: cc,
	}
}

func (server *Server) ServeHTTP()
