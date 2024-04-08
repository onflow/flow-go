package ping

import (
	"context"
	"encoding/binary"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
)

const (
	// PingTimeout is maximum time to wait for a ping reply from a remote node
	PingTimeout = time.Second * 4

	// PingInterval is the interval between pings to remote nodes
	PingInterval = time.Minute
)

type Engine struct {
	component.Component

	log          zerolog.Logger
	idProvider   module.IdentityProvider
	idTranslator p2p.IDTranslator
	me           module.Local
	metrics      module.PingMetrics

	pingService network.PingService
	nodeInfo    map[flow.Identifier]string // additional details about a node such as operator name
}

func New(
	log zerolog.Logger,
	idProvider module.IdentityProvider,
	idTranslator p2p.IDTranslator,
	me module.Local,
	metrics module.PingMetrics,
	nodeInfoFile string,
	pingService network.PingService,
) (*Engine, error) {
	eng := &Engine{
		log:          log.With().Str("engine", "ping").Logger(),
		idProvider:   idProvider,
		idTranslator: idTranslator,
		me:           me,
		metrics:      metrics,
		pingService:  pingService,
	}
	eng.nodeInfo = eng.loadNodeInfo(nodeInfoFile)

	eng.Component = component.NewComponentManagerBuilder().
		AddWorker(eng.pingLoop).
		Build()

	return eng, nil
}

func (e *Engine) loadNodeInfo(nodeInfoFile string) map[flow.Identifier]string {
	if nodeInfoFile == "" {
		// initialize nodeInfo with an empty map
		// the node info file is not mandatory and should not stop the Ping engine from running
		e.log.Trace().Msg("no node info file specified")
		return make(map[flow.Identifier]string)
	}

	nodeInfo, err := readExtraNodeInfoJSON(nodeInfoFile)
	if err != nil {
		e.log.Error().Err(err).
			Str("node_info_file", nodeInfoFile).
			Msg("failed to read node info file")
		return make(map[flow.Identifier]string)
	}

	e.log.Debug().
		Str("node_info_file", nodeInfoFile).
		Msg("using node info file")
	return nodeInfo
}

func (e *Engine) pingLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ticker := time.NewTicker(PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.pingAllNodes(ctx)
		}
	}
}

func (e *Engine) pingAllNodes(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, PingTimeout)
	defer cancel()

	wg := sync.WaitGroup{}

	peers := e.idProvider.Identities(filter.Not(filter.HasNodeID[flow.Identity](e.me.NodeID())))
	for _, peer := range peers {
		wg.Add(1)
		go func(peer *flow.Identity) {
			defer wg.Done()

			pid := peer.ID()
			delay := time.Duration(binary.BigEndian.Uint16(pid[:2])) % (PingInterval / time.Millisecond)

			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}

			e.pingNode(ctx, peer)
		}(peer)
	}

	wg.Wait()
}

// pingNode pings the given peer and updates the metrics with the result and the additional node information
func (e *Engine) pingNode(ctx context.Context, peer *flow.Identity) {
	pid, err := e.idTranslator.GetPeerID(peer.ID())

	if err != nil {
		e.log.Error().Err(err).Str("peer", peer.String()).Msg("failed to get peer ID")
		return
	}

	// ping the node
	resp, rtt, pingErr := e.pingService.Ping(ctx, pid) // ping will timeout in PingTimeout seconds
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
