package forwarder

import (
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	rpcConnection "github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/grpcutils"

	"github.com/onflow/flow/protobuf/go/flow/access"
)

// Forwarder forwards all requests to a set of upstream access nodes or observers
type Forwarder struct {
	lock        sync.Mutex
	roundRobin  int
	ids         flow.IdentityList
	upstream    []access.AccessAPIClient
	connections []*grpc.ClientConn
	timeout     time.Duration
	maxMsgSize  uint
}

func NewForwarder(identities flow.IdentityList, timeout time.Duration, maxMsgSize uint) (*Forwarder, error) {
	forwarder := &Forwarder{maxMsgSize: maxMsgSize}
	err := forwarder.setFlowAccessAPI(identities, timeout)
	return forwarder, err
}

// setFlowAccessAPI sets a backend access API that forwards some requests to an upstream node.
// It is used by Observer services, Blockchain Data Service, etc.
// Make sure that this is just for observation and not a staked participant in the flow network.
// This means that observers see a copy of the data but there is no interaction to ensure integrity from the root block.
func (f *Forwarder) setFlowAccessAPI(accessNodeAddressAndPort flow.IdentityList, timeout time.Duration) error {
	f.timeout = timeout
	f.ids = accessNodeAddressAndPort
	f.upstream = make([]access.AccessAPIClient, accessNodeAddressAndPort.Count())
	f.connections = make([]*grpc.ClientConn, accessNodeAddressAndPort.Count())
	for i, identity := range accessNodeAddressAndPort {
		// Store the faultTolerantClient setup parameters such as address, public, key and timeout, so that
		// we can refresh the API on connection loss
		f.ids[i] = identity

		// We fail on any single error on startup, so that
		// we identify bootstrapping errors early
		err := f.reconnectingClient(i)
		if err != nil {
			return err
		}
	}

	f.roundRobin = 0
	return nil
}

// reconnectingClient returns an active client, or
// creates one, if the last one is not ready anymore.
func (f *Forwarder) reconnectingClient(i int) error {
	timeout := f.timeout

	if f.connections[i] == nil || f.connections[i].GetState() != connectivity.Ready {
		identity := f.ids[i]
		var connection *grpc.ClientConn
		var err error
		if identity.NetworkPubKey == nil {
			connection, err = grpc.Dial(
				identity.Address,
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(int(f.maxMsgSize))),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				rpcConnection.WithClientTimeoutOption(timeout))
			if err != nil {
				return err
			}
		} else {
			tlsConfig, err := grpcutils.DefaultClientTLSConfig(identity.NetworkPubKey)
			if err != nil {
				return fmt.Errorf("failed to get default TLS client config using public flow networking key %s %w", identity.NetworkPubKey.String(), err)
			}

			connection, err = grpc.Dial(
				identity.Address,
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(int(f.maxMsgSize))),
				grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
				rpcConnection.WithClientTimeoutOption(timeout))
			if err != nil {
				return fmt.Errorf("cannot connect to %s %w", identity.Address, err)
			}
		}
		connection.Connect()
		time.Sleep(1 * time.Second)
		state := connection.GetState()
		if state != connectivity.Ready && state != connectivity.Connecting {
			return fmt.Errorf("%v", state)
		}
		f.connections[i] = connection
		f.upstream[i] = access.NewAccessAPIClient(connection)
	}

	return nil
}

// FaultTolerantClient implements an upstream connection that reconnects on errors
// a reasonable amount of time.
func (f *Forwarder) FaultTolerantClient() (access.AccessAPIClient, error) {
	if f.upstream == nil || len(f.upstream) == 0 {
		return nil, status.Errorf(codes.Unimplemented, "method not implemented")
	}

	// Reasoning: A retry count of three gives an acceptable 5% failure ratio from a 37% failure ratio.
	// A bigger number is problematic due to the DNS resolve and connection times,
	// plus the need to log and debug each individual connection failure.
	//
	// This reasoning eliminates the need of making this parameter configurable.
	// The logic works rolling over a single connection as well making clean code.
	const retryMax = 3

	f.lock.Lock()
	defer f.lock.Unlock()

	var err error
	for i := 0; i < retryMax; i++ {
		f.roundRobin++
		f.roundRobin = f.roundRobin % len(f.upstream)
		err = f.reconnectingClient(f.roundRobin)
		if err != nil {
			continue
		}
		state := f.connections[f.roundRobin].GetState()
		if state != connectivity.Ready && state != connectivity.Connecting {
			continue
		}
		return f.upstream[f.roundRobin], nil
	}

	return nil, status.Errorf(codes.Unavailable, err.Error())
}
