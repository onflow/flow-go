// Package libp2p encapsulates the libp2p library
package p2p

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/libp2p/go-libp2p-core/crypto"
	libp2pnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	swarm "github.com/libp2p/go-libp2p-swarm"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"

	flownet "github.com/onflow/flow-go/network"
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
	PubKey crypto.PubKey
}

// Node is a wrapper around LibP2P host.
type Node struct {
	sync.Mutex
	libP2PHost *LibP2PHost                     // reference to the libp2p host (https://godoc.org/github.com/libp2p/go-libp2p-core/host)
	logger     zerolog.Logger                  // for logging
	topics     map[string]*pubsub.Topic        // map of a topic string to an actual topic instance
	subs       map[string]*pubsub.Subscription // map of a topic string to an actual subscription
}

func NewLibP2PNode(logger zerolog.Logger, libp2pHost *LibP2PHost) (*Node, error) {
	n := &Node{
		logger:     logger,
		topics:     make(map[string]*pubsub.Topic),
		subs:       make(map[string]*pubsub.Subscription),
		libP2PHost: libp2pHost,
	}

	ip, port, err := n.GetIPPort()
	if err != nil {
		return nil, fmt.Errorf("failed to find IP and port on which the node was started: %w", err)
	}

	n.logger.Debug().
		Str("name", n.libP2PHost.Name()).
		Str("address", fmt.Sprintf("%s:%s", ip, port)).
		Msg("libp2p node started successfully")

	return n, nil
}

// Stop stops the libp2p node.
func (n *Node) Stop() (chan struct{}, error) {
	var result error
	done := make(chan struct{})
	n.logger.Debug().Str("name", n.libP2PHost.Name()).Msg("unsubscribing from all topics")
	for t := range n.topics {
		if err := n.UnSubscribe(t); err != nil {
			result = multierror.Append(result, err)
		}
	}

	n.logger.Debug().Str("name", n.libP2PHost.Name()).Msg("stopping libp2p node")
	if err := n.libP2PHost.Host().Close(); err != nil {
		result = multierror.Append(result, err)
	}

	n.logger.Debug().Str("name", n.libP2PHost.Name()).Msg("closing peer store")
	// to prevent peerstore routine leak (https://github.com/libp2p/go-libp2p/issues/718)
	if err := n.libP2PHost.Host().Peerstore().Close(); err != nil {
		n.logger.Debug().Str("name", n.libP2PHost.Name()).Err(err).Msg("closing peer store")
		result = multierror.Append(result, err)
	}

	if result != nil {
		close(done)
		return done, result
	}

	go func(done chan struct{}) {
		defer close(done)
		addrs := len(n.libP2PHost.Host().Network().ListenAddresses())
		ticker := time.NewTicker(time.Millisecond * 2)
		defer ticker.Stop()
		timeout := time.After(time.Second)
		for addrs > 0 {
			// wait for all listen addresses to have been removed
			select {
			case <-timeout:
				n.logger.Error().Int("port", addrs).Msg("listen addresses still open")
				return
			case <-ticker.C:
				addrs = len(n.libP2PHost.Host().Network().ListenAddresses())
			}
		}
		n.logger.Debug().Str("name", n.libP2PHost.Name()).Msg("libp2p node stopped successfully")
	}(done)

	return done, nil
}

// AddPeer adds a peer to this node by adding it to this node's peerstore and connecting to it
func (n *Node) AddPeer(ctx context.Context, peer NodeAddress) error {
	pInfo, err := GetPeerInfo(peer)
	if err != nil {
		return fmt.Errorf("failed to add peer %s: %w", peer.Name, err)
	}

	err = n.libP2PHost.Host().Connect(ctx, pInfo)
	if err != nil {
		return err
	}

	return nil
}

// RemovePeer closes the connection with the peer
func (n *Node) RemovePeer(ctx context.Context, peer NodeAddress) error {
	pInfo, err := GetPeerInfo(peer)
	if err != nil {
		return fmt.Errorf("failed to remove peer %s: %w", peer.Name, err)
	}

	err = n.libP2PHost.Host().Network().ClosePeer(pInfo.ID)
	if err != nil {
		return fmt.Errorf("failed to remove peer %s: %w", peer.Name, err)
	}
	return nil
}

// CreateStream returns an existing stream connected to n if it exists or adds node n as a peer and creates a new stream with it
func (n *Node) CreateStream(ctx context.Context, nodeAddress NodeAddress) (libp2pnet.Stream, error) {

	// Get the PeerID
	peerID, err := peer.IDFromPublicKey(nodeAddress.PubKey)
	if err != nil {
		return nil, fmt.Errorf("could not get peer ID: %w", err)
	}

	// Open libp2p Stream with the remote peer (will use an existing TCP connection underneath if it exists)
	stream, err := n.tryCreateNewStream(ctx, nodeAddress, peerID, maxConnectAttempt)
	if err != nil {
		return nil, flownet.NewPeerUnreachableError(fmt.Errorf("could not create stream (name: %s, address: %s:%s): %w", nodeAddress.Name, nodeAddress.IP,
			nodeAddress.Port,
			err))
	}
	return stream, nil
}

// tryCreateNewStream makes at most maxAttempts to create a stream with the target peer
// This was put in as a fix for #2416. PubSub and 1-1 communication compete with each other when trying to connect to
// remote nodes and once in a while NewStream returns an error 'both yamux endpoints are clients'
func (n *Node) tryCreateNewStream(ctx context.Context, nodeAddress NodeAddress, targetID peer.ID, maxAttempts int) (libp2pnet.Stream, error) {
	var errs, err error
	var s libp2pnet.Stream
	var retries = 0
	for ; retries < maxAttempts; retries++ {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context done before stream could be created (retry attempt: %d", retries)
		default:
		}

		// remove the peer from the peer store if present
		n.libP2PHost.Host().Peerstore().ClearAddrs(targetID)

		// cancel the dial back off (if any), since we want to connect immediately
		network := n.libP2PHost.Host().Network()
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
		err = n.AddPeer(ctx, nodeAddress)
		if err != nil {

			// if the connection was rejected due to invalid node id, skip the re-attempt
			if strings.Contains(err.Error(), "failed to negotiate security protocol") {
				return s, fmt.Errorf("invalid node id: %w", err)
			}

			// if the connection was rejected due to allowlisting, skip the re-attempt
			if errors.Is(err, swarm.ErrGaterDisallowedConnection) {
				return s, fmt.Errorf("target node is not on the approved list of nodes: %w", err)
			}

			errs = multierror.Append(errs, err)
			continue
		}

		s, err = n.libP2PHost.Host().NewStream(ctx, targetID, n.libP2PHost.FlowLibP2PProtocolID())
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
			return []peer.AddrInfo{}, err
		}
	}
	return peerInfos, err
}

// GetIPPort returns the IP and Port the libp2p node is listening on.
func (n *Node) GetIPPort() (string, string, error) {
	return IPPortFromMultiAddress(n.libP2PHost.Host().Network().ListenAddresses()...)
}

// Subscribe subscribes the node to the given topic and returns the subscription
// Currently only one subscriber is allowed per topic.
// NOTE: A node will receive its own published messages.
func (n *Node) Subscribe(ctx context.Context, topic string) (*pubsub.Subscription, error) {
	n.Lock()
	defer n.Unlock()

	// Check if the topic has been already created and is in the cache
	n.libP2PHost.PubSub().GetTopics()
	tp, found := n.topics[topic]
	var err error
	if !found {
		tp, err = n.libP2PHost.PubSub().Join(topic)
		if err != nil {
			return nil, fmt.Errorf("could not join topic (%s): %w", topic, err)
		}
		n.topics[topic] = tp
	}

	// Create a new subscription
	s, err := tp.Subscribe()
	if err != nil {
		return s, fmt.Errorf("could not subscribe to topic (%s): %w", topic, err)
	}

	// Add the subscription to the cache
	n.subs[topic] = s

	n.logger.Debug().Str("topic", topic).Str("name", n.libP2PHost.Name()).Msg("subscribed to topic")
	return s, err
}

// UnSubscribe cancels the subscriber and closes the topic.
func (n *Node) UnSubscribe(topic string) error {
	n.Lock()
	defer n.Unlock()
	// Remove the Subscriber from the cache
	if s, found := n.subs[topic]; found {
		s.Cancel()
		n.subs[topic] = nil
		delete(n.subs, topic)
	}

	tp, found := n.topics[topic]
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
	n.topics[topic] = nil
	delete(n.topics, topic)

	n.logger.Debug().Str("topic", topic).Str("name", n.libP2PHost.Name()).Msg("unsubscribed from topic")
	return err
}

// Publish publishes the given payload on the topic
func (n *Node) Publish(ctx context.Context, topic string, data []byte) error {
	ps, found := n.topics[topic]
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
func (n *Node) Ping(ctx context.Context, target NodeAddress) (time.Duration, error) {

	pingError := func(err error) (time.Duration, error) {
		return -1, fmt.Errorf("failed to ping %s (%s:%s): %w", target.Name, target.IP, target.Port, err)
	}

	// convert the target node address to libp2p peer info
	targetInfo, err := GetPeerInfo(target)
	if err != nil {
		return pingError(err)
	}

	// connect to the target node
	err = n.libP2PHost.Host().Connect(ctx, targetInfo)
	if err != nil {
		return pingError(err)
	}

	// create a cancellable ping context
	pctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ping the target
	resultChan := ping.Ping(pctx, n.libP2PHost.Host(), targetInfo.ID)

	// read the result channel
	select {
	case res := <-resultChan:
		if res.Error != nil {
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

// UpdateAllowlist allows the peer allowlist to be updated
func (n *Node) UpdateAllowlist(allowListAddrs ...NodeAddress) error {
	whilelistPInfos, err := GetPeerInfos(allowListAddrs...)
	if err != nil {
		return fmt.Errorf("failed to create approved list of peers: %w", err)
	}
	n.libP2PHost.ConnenctionGater().update(whilelistPInfos)
	return nil
}

// IsConnected returns true is address is a direct peer of this node else false
func (n *Node) IsConnected(address NodeAddress) (bool, error) {
	pInfo, err := GetPeerInfo(address)
	if err != nil {
		return false, err
	}
	// query libp2p for connectedness status of this peer
	isConnected := n.libP2PHost.Host().Network().Connectedness(pInfo.ID) == libp2pnet.Connected
	return isConnected, nil
}

func generateProtocolID(rootBlockID string) protocol.ID {
	return protocol.ID(FlowLibP2PProtocolIDPrefix + rootBlockID)
}
