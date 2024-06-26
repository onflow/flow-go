package forwarder

import (
	"fmt"
	"io"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow/protobuf/go/flow/access"
)

// Upstream is a container for an individual upstream containing the id, client and closer for it
type Upstream struct {
	id     *flow.IdentitySkeleton // the public identity of one network participant (node)
	client access.AccessAPIClient // client with gRPC connection
	closer io.Closer              // closer for client connection, should use to close the connection when done
}

// Forwarder forwards all requests to a set of upstream access nodes or observers
type Forwarder struct {
	lock        sync.Mutex
	roundRobin  int
	upstream    []Upstream
	connFactory connection.ConnectionFactory
}

func NewForwarder(identities flow.IdentitySkeletonList, connectionFactory connection.ConnectionFactory) (*Forwarder, error) {
	forwarder := &Forwarder{connFactory: connectionFactory}
	err := forwarder.setFlowAccessAPI(identities)
	return forwarder, err
}

// setFlowAccessAPI sets a backend access API that forwards some requests to an upstream node.
// It is used by Observer services, Blockchain Data Service, etc.
// Make sure that this is just for observation and not a staked participant in the flow network.
// This means that observers see a copy of the data but there is no interaction to ensure integrity from the root block.
func (f *Forwarder) setFlowAccessAPI(accessNodeAddressAndPort flow.IdentitySkeletonList) error {
	f.upstream = make([]Upstream, accessNodeAddressAndPort.Count())
	for i, identity := range accessNodeAddressAndPort {
		// Store the faultTolerantClient setup parameters such as address, public, key and timeout, so that
		// we can refresh the API on connection loss
		f.upstream[i].id = identity

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

// reconnectingClient returns an active client, or creates a new connection.
func (f *Forwarder) reconnectingClient(i int) error {
	identity := f.upstream[i].id

	accessApiClient, closer, err := f.connFactory.GetAccessAPIClientWithPort(identity.Address, identity.NetworkPubKey)
	if err != nil {
		return fmt.Errorf("failed to connect to access node at %s: %w", accessApiClient, err)
	}
	// closer is not nil iff err is nil, should use to close the connection when done
	f.upstream[i].closer = closer
	f.upstream[i].client = accessApiClient
	return nil
}

// FaultTolerantClient implements an upstream connection that reconnects on errors
// a reasonable amount of time.
func (f *Forwarder) FaultTolerantClient() (access.AccessAPIClient, io.Closer, error) {
	if f.upstream == nil || len(f.upstream) == 0 {
		return nil, nil, status.Errorf(codes.Unimplemented, "method not implemented")
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
		return f.upstream[f.roundRobin].client, f.upstream[f.roundRobin].closer, nil
	}

	return nil, nil, status.Errorf(codes.Unavailable, err.Error())
}
