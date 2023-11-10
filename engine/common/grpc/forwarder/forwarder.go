package forwarder

import (
	"fmt"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/model/flow"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"sync"

	"github.com/onflow/flow/protobuf/go/flow/access"
)

// Forwarder forwards all requests to a set of upstream access nodes or observers
type Forwarder struct {
	lock        sync.Mutex
	roundRobin  int
	ids         flow.IdentityList
	upstream    []access.AccessAPIClient
	closer      []io.Closer
	connFactory connection.ConnectionFactory
}

func NewForwarder(identities flow.IdentityList, connectionFactory connection.ConnectionFactory) (*Forwarder, error) {
	forwarder := &Forwarder{connFactory: connectionFactory}
	err := forwarder.setFlowAccessAPI(identities)
	return forwarder, err
}

// setFlowAccessAPI sets a backend access API that forwards some requests to an upstream node.
// It is used by Observer services, Blockchain Data Service, etc.
// Make sure that this is just for observation and not a staked participant in the flow network.
// This means that observers see a copy of the data but there is no interaction to ensure integrity from the root block.
func (f *Forwarder) setFlowAccessAPI(accessNodeAddressAndPort flow.IdentityList) error {
	f.ids = accessNodeAddressAndPort
	f.upstream = make([]access.AccessAPIClient, accessNodeAddressAndPort.Count())
	f.closer = make([]io.Closer, accessNodeAddressAndPort.Count())
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
	identity := f.ids[i]
	fmt.Println(fmt.Sprintf("identity.Address: %v", identity.Address))
	fmt.Println(fmt.Sprintf("identity.NetworkPubKey: %v", identity.NetworkPubKey))

	accessApiClient, closer, err := f.connFactory.GetAccessAPIClient(identity.Address, identity.NetworkPubKey)
	if err != nil {
		return fmt.Errorf("failed to connect to access node at %s: %w", accessApiClient, err)
	}
	f.closer[i] = closer
	f.upstream[i] = accessApiClient
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
			fmt.Println(fmt.Sprintf("+++FaultTolerantClient err: %v", err))
			continue
		}
		fmt.Println(fmt.Sprintf("FaultTolerantClient ok"))
		return f.upstream[f.roundRobin], f.closer[f.roundRobin], nil
	}

	return nil, nil, status.Errorf(codes.Unavailable, err.Error())
}
