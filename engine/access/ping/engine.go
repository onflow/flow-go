package ping

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

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

	// MaxConcurrentPings is the maximum number of ping requests that can be sent concurrently
	MaxConcurrentPings = 100

	// MaxJitter is the maximum time to pause between nodes during ping
	MaxJitter = 5 * time.Second
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
	start := time.Now()
	e.log.Debug().Msg("pinging all nodes")

	g := new(errgroup.Group)

	// restrict the number of concurrently running ping requests.
	g.SetLimit(MaxConcurrentPings)

	peers := e.idProvider.Identities(filter.Not(filter.HasNodeID[flow.Identity](e.me.NodeID())))
	for i, peer := range peers {
		peer := peer
		delay := makeJitter(i)

		g.Go(func() error {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(delay):
			}

			e.pingNode(ctx, peer)
			return nil
		})
	}

	_ = g.Wait()

	e.log.Debug().
		Dur("duration", time.Since(start)).
		Int("node_count", len(peers)).
		Msg("finished pinging all nodes")
}

// pingNode pings the given peer and updates the metrics with the result and the additional node information
func (e *Engine) pingNode(ctx context.Context, peer *flow.Identity) {
	pid, err := e.idTranslator.GetPeerID(peer.ID())

	if err != nil {
		e.log.Error().Err(err).Str("peer", peer.String()).Msg("failed to get peer ID")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, PingTimeout)
	defer cancel()

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

// makeJitter returns a jitter between 0 and MaxJitter
func makeJitter(offset int) time.Duration {
	jitter := float64(MaxJitter) * float64(offset%MaxConcurrentPings) / float64(MaxConcurrentPings)
	return time.Duration(jitter)
}
