// Package libp2p encapsulates the libp2p library
package libp2p

import (
	"context"
	"errors"
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
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	swarm "github.com/libp2p/go-libp2p-swarm"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"

	netwk "github.com/dapperlabs/flow-go/network"
)

const (
	// A unique Libp2p protocol ID prefix for Flow (https://docs.libp2p.io/concepts/protocols/)
	// All nodes communicate with each other using this protocol id suffixed with the id of the root block
	FlowLibP2PProtocolIDPrefix = "/flow/push/"

	// Maximum time to wait for a ping reply from a remote node
	PingTimeoutSecs = time.Second * 4
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
	name                 string                          // friendly human readable Name of the node
	libP2PHost           host.Host                       // reference to the libp2p host (https://godoc.org/github.com/libp2p/go-libp2p-core/host)
	logger               zerolog.Logger                  // for logging
	ps                   *pubsub.PubSub                  // the reference to the pubsub instance
	topics               map[string]*pubsub.Topic        // map of a topic string to an actual topic instance
	subs                 map[string]*pubsub.Subscription // map of a topic string to an actual subscription
	conMgr               ConnManager                     // the connection manager passed in to libp2p
	connGater            *connGater                      // the connection gator passed in to libp2p
	flowLibP2PProtocolID protocol.ID                     // the unique protocol ID
}

// Start starts a libp2p node on the given address.
func (p *P2PNode) Start(ctx context.Context,
	n NodeAddress,
	logger zerolog.Logger,
	key lcrypto.PrivKey,
	handler network.StreamHandler,
	rootBlockID string,
	whiteList bool,
	whitelistAddrs []NodeAddress,
	psOption ...pubsub.Option) error {
	p.Lock()
	defer p.Unlock()

	p.name = n.Name
	p.logger = logger
	p.flowLibP2PProtocolID = generateProtocolID(rootBlockID)
	addr := MultiaddressStr(n)
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

	// gather all the options for the libp2p node
	var options []config.Option
	options = append(options,
		libp2p.ListenAddrs(sourceMultiAddr), // set the listen address
		libp2p.Identity(key),                // pass in the networking key
		libp2p.ConnectionManager(p.conMgr),  // set the connection manager
		transport,                           // set the protocol
		libp2p.Ping(true),                   // enable ping
	)

	// if whitelisting is enabled, create a connection gator with whitelistAddrs
	if whiteList {

		// convert each of the whitelist address to libp2p peer infos
		whilelistPInfos, err := GetPeerInfos(whitelistAddrs...)
		if err != nil {
			return fmt.Errorf("failed to create approved list of peers: %w", err)
		}

		// create a connection gater
		p.connGater = newConnGater(whilelistPInfos, p.logger)

		// provide the connection gater as an option to libp2p
		options = append(options, libp2p.ConnectionGater(p.connGater))
	}

	// create the libp2p host
	host, err := libp2p.New(ctx, options...)
	if err != nil {
		return fmt.Errorf("could not create libp2p host: %w", err)
	}

	p.libP2PHost = host

	host.SetStreamHandler(p.flowLibP2PProtocolID, handler)

	// Creating a new PubSub instance of the type GossipSub with psOption
	p.ps, err = pubsub.NewGossipSub(ctx, p.libP2PHost, psOption...)
	if err != nil {
		return fmt.Errorf("could not create libp2p pubsub: %w", err)
	}

	p.topics = make(map[string]*pubsub.Topic)
	p.subs = make(map[string]*pubsub.Subscription)

	ip, port, err := p.GetIPPort()
	if err != nil {
		return fmt.Errorf("failed to find IP and port on which the node was started: %w", err)
	}

	p.logger.Debug().
		Str("name", p.name).
		Str("address", fmt.Sprintf("%s:%s", ip, port)).
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

	p.logger.Debug().Str("name", p.name).Msg("stopping libp2p node")
	if err := p.libP2PHost.Close(); err != nil {
		result = multierror.Append(result, err)
	}

	p.logger.Debug().Str("name", p.name).Msg("closing peer store")
	// to prevent peerstore routine leak (https://github.com/libp2p/go-libp2p/issues/718)
	if err := p.libP2PHost.Peerstore().Close(); err != nil {
		p.logger.Debug().Str("name", p.name).Err(err).Msg("closing peer store")
		result = multierror.Append(result, err)
	}

	if result != nil {
		close(done)
		return done, result
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
		return nil, netwk.NewPeerUnreachableError(fmt.Errorf("could not create stream (name: %s, address: %s:%s): %w", n.Name, n.IP, n.Port, err))
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

		// remove the peer from the peer store if present
		p.libP2PHost.Peerstore().ClearAddrs(targetID)

		// cancel the dial back off (if any), since we want to connect immediately
		network := p.libP2PHost.Network()
		if swm, ok := network.(*swarm.Swarm); ok {
			swm.Backoff().Clear(targetID)
		}

		// if this is a retry attempt, wait for some time before retrying
		if retries > 0 {
			// choose a random interval between 0 and 5 ms to retry
			r := rand.Intn(5)
			time.Sleep(time.Duration(r) * time.Millisecond)
		}

		// add node address as a peer
		err = p.AddPeers(ctx, n)
		if err != nil {
			// if the connection was rejected due to whitelisting, skip the re-attempt
			if errors.Is(err, swarm.ErrGaterDisallowedConnection) {
				return s, fmt.Errorf("target node is not on the approved list of nodes: %w", err)
			}
			errs = multierror.Append(errs, err)
			continue
		}

		s, err = p.libP2PHost.NewStream(ctx, targetID, p.flowLibP2PProtocolID)
		if err != nil {
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
	addr := MultiaddressStr(p)
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

func GetPeerInfos(addrs ...NodeAddress) ([]peer.AddrInfo, error) {
	peerInfos := make([]peer.AddrInfo, len(addrs))
	var err error
	for i, addr := range addrs {
		peerInfos[i], err = GetPeerInfo(addr)
		if err != nil {
			break
		}
	}
	return peerInfos, err
}

// GetIPPort returns the IP and Port the libp2p node is listening on.
func (p *P2PNode) GetIPPort() (string, string, error) {
	return IPPortFromMultiAddress(p.libP2PHost.Network().ListenAddresses()...)
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
			return nil, fmt.Errorf("could not join topic (%s): %w", topic, err)
		}
		p.topics[topic] = tp
	}

	// Create a new subscription
	s, err := tp.Subscribe()
	if err != nil {
		return s, fmt.Errorf("could not subscribe to topic (%s): %w", topic, err)
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
		err := fmt.Errorf("could not find topic (%s)", topic)
		return err
	}

	// attempt to close the topic
	err := tp.Close()
	if err != nil {
		err = fmt.Errorf("could not close topic (%s): %w", topic, err)
		return err
	}
	p.topics[topic] = nil
	delete(p.topics, topic)

	p.logger.Debug().Str("topic", topic).Str("name", p.name).Msg("unsubscribed from topic")
	return err
}

// Publish publishes the given payload on the topic
func (p *P2PNode) Publish(ctx context.Context, topic string, data []byte) error {
	ps, found := p.topics[topic]
	if !found {
		return fmt.Errorf("could not find topic (%s)", topic)
	}
	err := ps.Publish(ctx, data)
	if err != nil {
		return fmt.Errorf("could not publish top topic (%s): %w", topic, err)
	}
	return nil
}

// Ping pings a remote node and returns the time it took to ping the remote node if successful or the error
func (p *P2PNode) Ping(ctx context.Context, target NodeAddress) (time.Duration, error) {

	pingError := func(err error) (time.Duration, error) {
		return -1, fmt.Errorf("failed to ping %s (%s:%s): %w", target.Name, target.IP, target.Port, err)
	}

	// convert the target node address to libp2p peer info
	targetInfo, err := GetPeerInfo(target)
	if err != nil {
		return pingError(err)
	}

	// connect to the target node
	err = p.libP2PHost.Connect(ctx, targetInfo)
	if err != nil {
		return pingError(err)
	}

	// create a ping client
	pingClient := ping.NewPingService(p.libP2PHost)

	// ping the target
	resultChan := pingClient.Ping(ctx, targetInfo.ID)

	// read the result channel
	select {
	case res := <-resultChan:
		if err != nil {
			return pingError(err)
		}
		return res.RTT, nil
	case <-time.After(PingTimeoutSecs):
		return pingError(fmt.Errorf("timed out after %d seconds", PingTimeoutSecs))
	}
}

// MultiaddressStr receives a node address and returns
// its corresponding Libp2p Multiaddress in string format
// in current implementation IP part of the node address is
// either an IP or a dns4
// https://docs.libp2p.io/concepts/addressing/
func MultiaddressStr(address NodeAddress) string {
	parsedIP := net.ParseIP(address.IP)
	if parsedIP != nil {
		// returns parsed ip version of the multi-address
		return fmt.Sprintf("/ip4/%s/tcp/%s", address.IP, address.Port)
	}
	// could not parse it as an IP address and returns the dns version of the
	// multi-address
	return fmt.Sprintf("/dns4/%s/tcp/%s", address.IP, address.Port)
}

// IPPortFromMultiAddress returns the IP/hostname and the port for the given multi-addresses
// associated with a libp2p host
func IPPortFromMultiAddress(addrs ...multiaddr.Multiaddr) (string, string, error) {

	var ipOrHostname, port string
	var err error

	for _, a := range addrs {
		// try and get the dns4 hostname
		ipOrHostname, err = a.ValueForProtocol(multiaddr.P_DNS4)
		if err != nil {
			// if dns4 hostname is not found, try and get the IP address
			ipOrHostname, err = a.ValueForProtocol(multiaddr.P_IP4)
			if err != nil {
				continue // this may not be a TCP IP multiaddress
			}
		}

		// if either IP address or hostname is found, look for the port number
		port, err = a.ValueForProtocol(multiaddr.P_TCP)
		if err != nil {
			// an IPv4 or DNS4 based multiaddress should have a port number
			return "", "", err
		}

		//there should only be one valid IPv4 address
		return ipOrHostname, port, nil
	}
	return "", "", fmt.Errorf("ip address or hostname not found")
}

// UpdateWhitelist allows the peer whitelist to be updated
func (p *P2PNode) UpdateWhitelist(whitelistAddrs ...NodeAddress) error {
	whilelistPInfos, err := GetPeerInfos(whitelistAddrs...)
	if err != nil {
		return fmt.Errorf("failed to create approved list of peers: %w", err)
	}
	p.connGater.update(whilelistPInfos)
	return nil
}

func generateProtocolID(rootBlockID string) protocol.ID {
	return protocol.ID(FlowLibP2PProtocolIDPrefix + rootBlockID)
}
