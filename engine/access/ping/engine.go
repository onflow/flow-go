package ping

import (
	"context"
	"encoding/binary"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/network/p2p"
)

// PingTimeout is maximum time to wait for a ping reply from a remote node
const PingTimeout = time.Second * 4

type Engine struct {
	unit         *engine.Unit
	log          zerolog.Logger
	idProvider   id.IdentityProvider
	idTranslator p2p.IDTranslator
	me           module.Local
	metrics      module.PingMetrics

	pingEnabled  bool
	pingInterval time.Duration
	pingService  *p2p.PingService
	nodeInfo     map[flow.Identifier]string // additional details about a node such as operator name
}

func New(
	log zerolog.Logger,
	idProvider id.IdentityProvider,
	idTranslator p2p.IDTranslator,
	me module.Local,
	metrics module.PingMetrics,
	pingEnabled bool,
	pingService *p2p.PingService,
	nodeInfoFile string,
) (*Engine, error) {

	eng := &Engine{
		unit:         engine.NewUnit(),
		log:          log.With().Str("engine", "ping").Logger(),
		idProvider:   idProvider,
		idTranslator: idTranslator,
		me:           me,
		metrics:      metrics,
		pingEnabled:  pingEnabled,
		pingInterval: time.Minute,
		pingService:  pingService,
	}

	// if a node info file is provided, it is read and the additional node information is reported as part of the ping metric
	if nodeInfoFile != "" {
		nodeInfo, err := readExtraNodeInfoJSON(nodeInfoFile)
		if err != nil {
			log.Error().Err(err).Str("node_info_file", nodeInfoFile).Msg("failed to read node info file")
		} else {
			eng.nodeInfo = nodeInfo
			log.Debug().Str("node_info_file", nodeInfoFile).Msg("using node info file")
		}
	} else {
		// initialize nodeInfo with an empty map
		eng.nodeInfo = make(map[flow.Identifier]string)
		// the node info file is not mandatory and should not stop the Ping engine from running
		log.Trace().Msg("no node info file specified")
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
	pingInterval := time.Minute

	e.unit.LaunchPeriodically(func() {
		peers := e.idProvider.Identities(filter.Not(filter.HasNodeID(e.me.NodeID())))

		// for each peer, send a ping every ping interval
		for _, peer := range peers {
			peer := peer
			pid := peer.ID()
			delay := time.Duration(binary.BigEndian.Uint16(pid[:2])) % (pingInterval / time.Millisecond)
			e.unit.LaunchAfter(delay, func() {
				e.pingNode(peer)
			})
		}
	}, pingInterval, 0)
}

// pingNode pings the given peer and updates the metrics with the result and the additional node information
func (e *Engine) pingNode(peer *flow.Identity) {
	pid, err := e.idTranslator.GetPeerID(peer.ID())

	if err != nil {
		e.log.Error().Err(err).Str("peer", peer.String()).Msg("failed to get peer ID")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), PingTimeout)
	defer cancel()

	// ping the node
	resp, rtt, pingErr := e.pingService.Ping(ctx, pid) // ping will timeout in libp2p.PingTimeout seconds
	if pingErr != nil {
		e.log.Debug().Err(pingErr).Str("target", peer.ID().String()).Msg("failed to ping")
		// report the rtt duration as negative to make it easier to distinguish between pingable and non-pingable nodes
		rtt = -1
	}

	// get the additional info about the node
	info := e.nodeInfo[peer.ID()]

	// update metric
	e.metrics.NodeReachable(peer, info, rtt)

	// if ping succeeded then update the node info metric
	if pingErr == nil {
		e.metrics.NodeInfo(peer, info, resp.Version, resp.BlockHeight, resp.HotstuffView)
	}
}
