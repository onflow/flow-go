package unicast

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	libp2pnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	swarm "github.com/libp2p/go-libp2p-swarm"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
)

// MaxConnectAttemptSleepDuration is the maximum number of milliseconds to wait between attempts for a 1-1 direct connection
const MaxConnectAttemptSleepDuration = 5

// Manager manages libp2p stream negotiation and creation, which is utilized for unicast dispatches.
type Manager struct {
	logger         zerolog.Logger
	streamFactory  StreamFactory
	unicasts       []Protocol
	defaultHandler libp2pnet.StreamHandler
	sporkId        flow.Identifier
}

func NewUnicastManager(logger zerolog.Logger, streamFactory StreamFactory, sporkId flow.Identifier) *Manager {
	return &Manager{
		logger:        logger,
		streamFactory: streamFactory,
		sporkId:       sporkId,
	}
}

// WithDefaultHandler sets the default stream handler for this unicast manager. The default handler is utilized
// as the core handler for other unicast protocols, e.g., compressions.
func (m *Manager) WithDefaultHandler(defaultHandler libp2pnet.StreamHandler) {
	defaultProtocolID := FlowProtocolID(m.sporkId)
	m.defaultHandler = defaultHandler

	if len(m.unicasts) > 0 {
		panic("default handler must be set only once before any unicast registration")
	}

	m.unicasts = []Protocol{
		&PlainStream{
			protocolId: defaultProtocolID,
			handler:    defaultHandler,
		},
	}

	m.streamFactory.SetStreamHandler(defaultProtocolID, defaultHandler)
}

// Register registers given protocol name as preferred unicast. Each invocation of register prioritizes the current protocol
// over previously registered ones.
func (m *Manager) Register(unicast ProtocolName) error {
	factory, err := ToProtocolFactory(unicast)
	if err != nil {
		return fmt.Errorf("could not translate protocol name into factory: %w", err)
	}

	u := factory(m.logger, m.sporkId, m.defaultHandler)

	m.unicasts = append(m.unicasts, u)
	m.streamFactory.SetStreamHandler(u.ProtocolId(), u.Handler())

	return nil
}

// CreateStream makes at most `maxAttempts` to create a stream with the peer.
// This was put in as a fix for #2416. PubSub and 1-1 communication compete with each other when trying to connect to
// remote nodes and once in a while NewStream returns an error 'both yamux endpoints are clients'.
//
// Note that in case an existing TCP connection underneath to `peerID` exists, that connection is utilized for creating a new stream.
// The multiaddr.Multiaddr return value represents the addresses of `peerID` we dial while trying to create a stream to it.
func (m *Manager) CreateStream(ctx context.Context, peerID peer.ID, maxAttempts int) (libp2pnet.Stream, []multiaddr.Multiaddr, error) {
	var errs error

	for i := len(m.unicasts) - 1; i >= 0; i-- {
		s, addrs, err := m.createStreamWithProtocol(ctx, m.unicasts[i].ProtocolId(), peerID, maxAttempts)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}

		// return first successful stream
		return s, addrs, nil
	}

	return nil, nil, fmt.Errorf("could not create stream on any available unicast protocol: %w", errs)
}

// createStreamWithProtocol creates a stream on specified protocol.
func (m *Manager) createStreamWithProtocol(ctx context.Context,
	protocolID protocol.ID,
	peerID peer.ID,
	maxAttempts int) (libp2pnet.Stream, []multiaddr.Multiaddr, error) {

	var errs error
	var s libp2pnet.Stream
	var retries = 0
	var dialAddr []multiaddr.Multiaddr // address on which we dial peerID
	for ; retries < maxAttempts; retries++ {
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("context done before stream could be created (retry attempt: %d, errors: %w)", retries, errs)
		default:
		}

		// libp2p internally uses swarm dial - https://github.com/libp2p/go-libp2p-swarm/blob/master/swarm_dial.go
		// to connect to a peer. Swarm dial adds a back off each time it fails connecting to a peer. While this is
		// the desired behaviour for pub-sub (1-k style of communication) for 1-1 style we want to retry the connection
		// immediately without backing off and fail-fast.
		// Hence, explicitly cancel the dial back off (if any) and try connecting again

		// cancel the dial back off (if any), since we want to connect immediately
		dialAddr = m.streamFactory.DialAddress(peerID)
		m.streamFactory.ClearBackoff(peerID)

		// if this is a retry attempt, wait for some time before retrying
		if retries > 0 {
			// choose a random interval between 0 to 5
			// (to ensure that this node and the target node don't attempt to reconnect at the same time)
			r := rand.Intn(MaxConnectAttemptSleepDuration)
			time.Sleep(time.Duration(r) * time.Millisecond)
		}

		err := m.streamFactory.Connect(ctx, peer.AddrInfo{ID: peerID})
		if err != nil {

			// if the connection was rejected due to invalid node id, skip the re-attempt
			if strings.Contains(err.Error(), "failed to negotiate security protocol") {
				return s, dialAddr, fmt.Errorf("invalid node id: %w", err)
			}

			// if the connection was rejected due to allowlisting, skip the re-attempt
			if errors.Is(err, swarm.ErrGaterDisallowedConnection) {
				return s, dialAddr, fmt.Errorf("target node is not on the approved list of nodes: %w", err)
			}

			errs = multierror.Append(errs, err)
			continue
		}

		// creates stream using stream factory
		s, err = m.streamFactory.NewStream(ctx, peerID, protocolID)
		if err != nil {
			// if the stream creation failed due to invalid protocol id, skip the re-attempt
			if strings.Contains(err.Error(), "protocol not supported") {
				return nil, dialAddr, fmt.Errorf("remote node is running on a different spork: %w, protocol attempted: %s", err, protocolID)
			}
			errs = multierror.Append(errs, err)
			continue
		}

		break
	}

	if retries == maxAttempts {
		return s, dialAddr, errs
	}

	return s, dialAddr, nil
}
