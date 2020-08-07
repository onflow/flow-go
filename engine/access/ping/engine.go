package ping

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
	"github.com/dapperlabs/flow-go/state/protocol"
)

type Engine struct {
	unit    *engine.Unit
	log     zerolog.Logger
	state   protocol.State
	me      module.Local
	metrics module.PingMetrics

	pingEnabled  bool
	pingInterval time.Duration
	middleware   *libp2p.Middleware
}

func New(
	log zerolog.Logger,
	state protocol.State,
	me module.Local,
	metrics module.PingMetrics,
	pingEnabled bool,
	mw *libp2p.Middleware,
) (*Engine, error) {
	eng := &Engine{
		unit:         engine.NewUnit(),
		log:          log.With().Str("engine", "ping").Logger(),
		state:        state,
		me:           me,
		metrics:      metrics,
		pingEnabled:  pingEnabled,
		pingInterval: time.Minute,
		middleware:   mw,
	}

	return eng, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the ingestion engine, we consider the engine up and running
// upon initialization.
func (e *Engine) Ready() <-chan struct{} {
	// only launch when ping is enabled
	if e.pingEnabled {
		e.unit.Launch(e.startPing)
	}
	e.log.Info().Bool("ping enabled", e.pingEnabled).Msg("ping enabled")
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the ingestion engine, it only waits for all submit goroutines to end.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

func (e *Engine) startPing() {
	peers, err := e.state.Final().Identities(filter.Not(filter.HasNodeID(e.me.NodeID())))
	if err != nil {
		e.log.Err(err).Msg("could not get identity list")
		return
	}

	pingInterval := time.Second * 60

	// for each peer, send a ping every ping interval
	for i, peer := range peers {
		func(peer *flow.Identity, delay time.Duration) {
			e.log.Info().Str("peer", peer.String()).Dur("interval", pingInterval).Msg("periodically ping node")
			e.unit.LaunchPeriodically(func() {
				e.pingNode(peer)
			}, pingInterval, delay)
		}(peer, time.Duration(i)*time.Second)
	}
}

// send ping to a given node and report the reachable result to metrics
func (e *Engine) pingNode(peer *flow.Identity) {
	reachable := e.pingAddress(peer.ID())
	e.metrics.NodeReachable(peer.ID(), reachable)
}

// pingAddress sends a ping request to the given address, and block until either receive
// a ping respond then return true, or hitting a timeout and return false.
// if there is other unknown error, return false
func (e *Engine) pingAddress(target flow.Identifier) bool {
	// ignore the ping duration for now
	// ping will timeout in libp2p.PingTimeoutSecs seconds
	_, err := e.middleware.Ping(target)
	if err != nil {
		e.log.Debug().Err(err).Str("target", target.String()).Msg("failed to ping")
		return false
	}
	return true
}
