// Package libp2p encapsulates the libp2p library
package libp2p

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/libp2p/go-libp2p"
	lcrypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	swarm "github.com/libp2p/go-libp2p-swarm"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	"github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// A unique Libp2p protocol ID for Flow (https://docs.libp2p.io/concepts/protocols/)
// All nodes communicate with each other using this protocol
const (
	FlowLibP2PProtocolID protocol.ID = "/flow/push/0.0.1"
)

// maximum number of attempts to be made to connect to a remote node for 1-1 direct communication
const maxConnectAttempt = 3

// NodeAddress is used to define a libp2p node
type NodeAddress struct {
	// Name is the friendly node Name e.g. "node1" (not to be confused with the libp2p node id)
	Name   string
	IP     string
	Port   string
	PubKey lcrypto.PubKey
}

// P2PNode manages the the libp2p node.
type P2PNode struct {
	sync.Mutex
	name       string                          // friendly human readable Name of the node
	libP2PHost host.Host                       // reference to the libp2p host (https://godoc.org/github.com/libp2p/go-libp2p-core/host)
	logger     zerolog.Logger                  // for logging
	ps         *pubsub.PubSub                  // the reference to the pubsub instance
	topics     map[string]*pubsub.Topic        // map of a topic string to an actual topic instance
	subs       map[string]*pubsub.Subscription // map of a topic string to an actual subscription
	conMgr     ConnManager                     // the connection manager passed in to libp2p
}

// Start starts a libp2p node on the given address.
func (p *P2PNode) Start(ctx context.Context, n NodeAddress, logger zerolog.Logger, key lcrypto.PrivKey, handler network.StreamHandler, psOption ...pubsub.Option) error {
	p.Lock()
	defer p.Unlock()

	p.name = n.Name
	p.logger = logger
	addr := multiaddressStr(n)
	sourceMultiAddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return err
	}

	p.conMgr = NewConnManager(logger)

	// create a transport which disables port reuse and web socket.
	// Port reuse enables listening and dialing from the same TCP port (https://github.com/libp2p/go-reuseport)
	// While this sounds great, it intermittently causes a 'broken pipe' error
	// as the 1-k discovery process and the 1-1 messaging both sometimes attempt to open connection to the same target
	// As of now there is no requirement of client sockets to be a well-known port, so disabling port reuse all together.
	transport := libp2p.Transport(func(u *tptu.Upgrader) *tcp.TcpTransport {
		tpt := tcp.NewTCPTransport(u)
		tpt.DisableReuseport = true
		return tpt
	})

	// libp2p.New constructs a new libp2p Host.
	// Other options can be added here.
	host, err := libp2p.New(
		ctx,
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.ConnectionManager(p.conMgr),
		//libp2p.NoSecurity,
		libp2p.Identity(key),
		transport,
	)
	if err != nil {
		return errors.Wrapf(err, "could not construct libp2p host for %s", p.name)
	}

	p.libP2PHost = host

	host.SetStreamHandler(FlowLibP2PProtocolID, handler)

	// Creating a new PubSub instance of the type GossipSub with psOption
	p.ps, err = pubsub.NewGossipSub(ctx, p.libP2PHost, psOption...)
	if err != nil {
		return errors.Wrapf(err, "unable to start pubsub %s", p.name)
	}

	p.topics = make(map[string]*pubsub.Topic)
	p.subs = make(map[string]*pubsub.Subscription)

	ip, port := p.GetIPPort()
	p.logger.Debug().Str("name", p.name).Str("address", fmt.Sprintf("%s:%s", ip, port)).
		Msg("libp2p node started successfully")

	return nil
}

// Stop stops the libp2p node.
func (p *P2PNode) Stop() (chan struct{}, error) {
	var result error
	done := make(chan struct{})
	p.logger.Debug().Str("name", p.name).Msg("unsubscribing from all topics")
	for t := range p.topics {
		if err := p.UnSubscribe(t); err != nil {
			result = multierror.Append(result, err)
		}
	}

	if result != nil {
		// TODO: Till #2485 - graceful shutdown is implemented, swallow all unsusbscribe errors and only log it
		p.logger.Debug().Err(result)
	}

	p.logger.Debug().Str("name", p.name).Msg("stopping libp2p node")
	if err := p.libP2PHost.Close(); err != nil {
		close(done)
		return done, multierror.Append(result, err)
	}

	go func(done chan struct{}) {
		defer close(done)
		addrs := len(p.libP2PHost.Network().ListenAddresses())
		ticker := time.NewTicker(time.Millisecond * 2)
		defer ticker.Stop()
		timeout := time.After(time.Second)
		for addrs > 0 {
			// wait for all listen addresses to have been removed
			select {
			case <-timeout:
				p.logger.Error().Int("port", addrs).Msg("listen addresses still open")
				return
			case <-ticker.C:
				addrs = len(p.libP2PHost.Network().ListenAddresses())
			}
		}
		p.logger.Debug().Str("name", p.name).Msg("libp2p node stopped successfully")
	}(done)

	return done, nil
}

// AddPeers adds other nodes as peers to this node by adding them to the node's peerstore and connecting to them
func (p *P2PNode) AddPeers(ctx context.Context, peers ...NodeAddress) error {
	p.Lock()
	defer p.Unlock()
	for _, peer := range peers {
		pInfo, err := GetPeerInfo(peer)
		if err != nil {
			return err
		}

		// Add the destination's peer multiaddress in the peerstore.
		// This will be used during connection and stream creation by libp2p.
		p.libP2PHost.Peerstore().AddAddrs(pInfo.ID, pInfo.Addrs, peerstore.PermanentAddrTTL)

		err = p.libP2PHost.Connect(ctx, pInfo)
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateStream returns an existing stream connected to n if it exists or adds node n as a peer and creates a new stream with it
func (p *P2PNode) CreateStream(ctx context.Context, n NodeAddress) (network.Stream, error) {

	// Get the PeerID
	peerID, err := peer.IDFromPublicKey(n.PubKey)
	if err != nil {
		return nil, fmt.Errorf("could not get peer ID: %w", err)
	}

	// Open libp2p Stream with the remote peer (will use an existing TCP connection underneath if it exists)
	stream, err := p.tryCreateNewStream(ctx, n, peerID, maxConnectAttempt)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream for %s: %w", peerID.String(), err)
	}
	return stream, nil
}

// tryCreateNewStream makes at most maxAttempts to create a stream with the target peer
// This was put in as a fix for #2416. PubSub and 1-1 communication compete with each other when trying to connect to
// remote nodes and once in a while NewStream returns an error 'both yamux endpoints are clients'
func (p *P2PNode) tryCreateNewStream(ctx context.Context, n NodeAddress, targetID peer.ID, maxAttempts int) (network.Stream, error) {
	var errs, err error
	var s network.Stream
	var retries = 0
	for ; retries < maxAttempts; retries++ {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context done before stream could be created (retry attempt: %d", retries)
		default:
		}

		// if this is a retry attempt, wait for some time before retrying
		if err != nil {
			// choose a random interval between 0 and 5 ms to retry
			r := rand.Intn(5)
			time.Sleep(time.Duration(r) * time.Millisecond)
			// cancel the dial back off, since we want to retry immediately
			n := p.libP2PHost.Network()
			if s, ok := n.(*swarm.Swarm); ok {
				s.Backoff().Clear(targetID)
			}
		}

		// Add node address as a peer
		err = p.AddPeers(ctx, n)
		if err != nil {
			p.logger.Error().Str("target", targetID.String()).Err(err).
				Int("retry_attempt", retries).Msg("could not create connection")
			errs = multierror.Append(errs, err)
			continue
		}

		s, err = p.libP2PHost.NewStream(ctx, targetID, FlowLibP2PProtocolID)
		if err != nil {
			p.logger.Error().Str("target", targetID.String()).Err(err).
				Int("retry_attempt", retries).Msg("failed to create stream")
			errs = multierror.Append(errs, err)
			continue
		}

		break
	}
	if retries == maxAttempts {
		return s, errs
	}
	return s, nil
}

// GetPeerInfo generates the libp2p peer.AddrInfo for a Node/Peer given its node address
func GetPeerInfo(p NodeAddress) (peer.AddrInfo, error) {
	addr := multiaddressStr(p)
	maddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	id, err := peer.IDFromPublicKey(p.PubKey)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	pInfo := peer.AddrInfo{ID: id, Addrs: []multiaddr.Multiaddr{maddr}}
	return pInfo, err
}

// GetIPPort returns the IP and Port the libp2p node is listening on.
func (p *P2PNode) GetIPPort() (ip string, port string) {
	for _, a := range p.libP2PHost.Network().ListenAddresses() {
		if ip, e := a.ValueForProtocol(multiaddr.P_IP4); e == nil {
			if p, e := a.ValueForProtocol(multiaddr.P_TCP); e == nil {
				return ip, p
			}
		}
	}
	return "", ""
}

// Subscribe subscribes the node to the given topic and returns the subscription
// Currently only one subscriber is allowed per topic.
// NOTE: A node will receive its own published messages.
func (p *P2PNode) Subscribe(ctx context.Context, topic string) (*pubsub.Subscription, error) {
	p.Lock()
	defer p.Unlock()

	// Check if the topic has been already created and is in the cache
	p.ps.GetTopics()
	tp, found := p.topics[topic]
	var err error
	if !found {
		tp, err = p.ps.Join(topic)
		if err != nil {
			return nil, fmt.Errorf("failed to join topic %s: %w", topic, err)
		}
		p.topics[topic] = tp
	}

	// Create a new subscription
	s, err := tp.Subscribe()
	if err != nil {
		return s, fmt.Errorf("failed to create subscription for topic %s: %w", topic, err)
	}

	// Add the subscription to the cache
	p.subs[topic] = s

	p.logger.Debug().Str("topic", topic).Str("name", p.name).Msg("subscribed to topic")
	return s, err
}

// UnSubscribe cancels the subscriber and closes the topic.
func (p *P2PNode) UnSubscribe(topic string) error {
	p.Lock()
	defer p.Unlock()
	// Remove the Subscriber from the cache
	if s, found := p.subs[topic]; found {
		s.Cancel()
		p.subs[topic] = nil
		delete(p.subs, topic)
	}

	tp, found := p.topics[topic]
	if !found {
		err := fmt.Errorf("topic %s not subscribed to", topic)
		return err
	}

	// attempt to close the topic
	err := tp.Close()
	if err != nil {
		err = errors.Wrapf(err, "unable to close topic %s", topic)
		return err
	}
	p.topics[topic] = nil
	delete(p.topics, topic)

	p.logger.Debug().Str("topic", topic).Str("name", p.name).Msg("unsubscribed from topic")
	return err
}

// Publish publishes the given payload on the topic
func (p *P2PNode) Publish(ctx context.Context, t string, data []byte) error {
	ps, found := p.topics[t]
	if !found {
		return fmt.Errorf("topic not found:%s", t)
	}
	err := ps.Publish(ctx, data)
	if err != nil {
		return fmt.Errorf("failed to publish to topic %s: %w", t, err)
	}
	return nil
}

// multiaddressStr receives a node address and returns
// its corresponding Libp2p Multiaddress in string format
// in current implementation IP part of the node address is
// either an IP or a dns4
// https://docs.libp2p.io/concepts/addressing/
func multiaddressStr(address NodeAddress) string {
	parsedIP := net.ParseIP(address.IP)
	if parsedIP != nil {
		// returns parsed ip version of the multi-address
		return fmt.Sprintf("/ip4/%s/tcp/%s", address.IP, address.Port)
	}
	// could not parse it as an IP address and returns the dns version of the
	// multi-address
	return fmt.Sprintf("/dns4/%s/tcp/%s", address.IP, address.Port)
}
