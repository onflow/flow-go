package apiproxy

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/grpcutils"
)

// Methodology
//
// We test the proxy and fall-over logic to reach basic coverage.
//
// * Basic coverage means that all conditional checks happen once but only once.
// * We embrace the simplest adequate solution to reduce engineering cost.
// * Any use cases requiring multiple conditionals exercised in a row are considered ignorable due to cost constraints.

// TestNetE2E tests the basic unix network first
func TestNetE2E(t *testing.T) {
	done := make(chan int)
	// Bring up 1st upstream server
	charlie1, err := makeFlowLite("/tmp/TestProxyE2E1", done)
	if err != nil {
		t.Fatal(err)
	}
	// Wait until proxy call passes
	err = callFlowLite("/tmp/TestProxyE2E1")
	if err != nil {
		t.Fatal(err)
	}
	// Bring up 2nd upstream server
	charlie2, err := makeFlowLite("/tmp/TestProxyE2E2", done)
	if err != nil {
		t.Fatal(err)
	}
	// Both proxy calls should pass
	err = callFlowLite("/tmp/TestProxyE2E1")
	if err != nil {
		t.Fatal(err)
	}
	err = callFlowLite("/tmp/TestProxyE2E2")
	if err != nil {
		t.Fatal(err)
	}
	// Stop 1st upstream server
	_ = charlie1.Close()
	// Proxy call falls through
	err = callFlowLite("/tmp/TestProxyE2E1")
	if err == nil {
		t.Fatal(fmt.Errorf("backend still active after close"))
	}
	// Stop 2nd upstream server
	_ = charlie2.Close()
	// System errors out on shut down servers
	err = callFlowLite("/tmp/TestProxyE2E1")
	if err == nil {
		t.Fatal(fmt.Errorf("backend still active after close"))
	}
	err = callFlowLite("/tmp/TestProxyE2E2")
	if err == nil {
		t.Fatal(fmt.Errorf("backend still active after close"))
	}
	// wait for all
	<-done
	<-done
}

// TestgRPCE2E tests whether gRPC works
func TestGRPCE2E(t *testing.T) {
	done := make(chan int)
	// Bring up 1st upstream server
	charlie1, _, err := newFlowLite("unix", "/tmp/TestProxyE2E1", done)
	if err != nil {
		t.Fatal(err)
	}
	// Wait until proxy call passes
	err = openFlowLite("/tmp/TestProxyE2E1")
	if err != nil {
		t.Fatal(err)
	}
	// Bring up 2nd upstream server
	charlie2, _, err := newFlowLite("unix", "/tmp/TestProxyE2E2", done)
	if err != nil {
		t.Fatal(err)
	}
	// Both proxy calls should pass
	err = openFlowLite("/tmp/TestProxyE2E1")
	if err != nil {
		t.Fatal(err)
	}
	err = openFlowLite("/tmp/TestProxyE2E2")
	if err != nil {
		t.Fatal(err)
	}
	// Stop 1st upstream server
	charlie1.Stop()
	// Proxy call falls through
	err = openFlowLite("/tmp/TestProxyE2E1")
	if err == nil {
		t.Fatal(fmt.Errorf("backend still active after close"))
	}
	// Stop 2nd upstream server
	charlie2.Stop()
	// System errors out on shut down servers
	err = openFlowLite("/tmp/TestProxyE2E1")
	if err == nil {
		t.Fatal(fmt.Errorf("backend still active after close"))
	}
	err = openFlowLite("/tmp/TestProxyE2E2")
	if err == nil {
		t.Fatal(fmt.Errorf("backend still active after close"))
	}
	// wait for all
	<-done
	<-done
}

// TestNewFlowCachedAccessAPIProxy tests the round robin end to end
func TestNewFlowCachedAccessAPIProxy(t *testing.T) {
	done := make(chan int)

	// Bring up 1st upstream server
	charlie1, _, err := newFlowLite("tcp", "127.0.0.1:11634", done)
	if err != nil {
		t.Fatal(err)
	}

	// Prepare a proxy that fails due to the second connection being idle
	l := flow.IdentityList{{Address: "127.0.0.1:11634"}, {Address: "127.0.0.1:11635"}}
	c, err := NewFlowAccessAPIProxy(l, time.Second)
	if err == nil {
		t.Fatal(fmt.Errorf("should not start with one connection ready"))
	}

	// Bring up 2nd upstream server
	charlie2, _, err := newFlowLite("tcp", "127.0.0.1:11635", done)
	if err != nil {
		t.Fatal(err)
	}

	background := context.Background()

	// Prepare a proxy
	l = flow.IdentityList{{Address: "127.0.0.1:11634"}, {Address: "127.0.0.1:11635"}}
	c, err = NewFlowAccessAPIProxy(l, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	// Wait until proxy call passes
	_, err = c.Ping(background, &access.PingRequest{})
	if err != nil {
		t.Fatal(err)
	}

	// Wait until proxy call passes
	_, err = c.Ping(background, &access.PingRequest{})
	if err != nil {
		t.Fatal(err)
	}

	// Wait until proxy call passes
	_, err = c.Ping(background, &access.PingRequest{})
	if err != nil {
		t.Fatal(err)
	}

	charlie1.Stop()
	charlie2.Stop()

	// Wait until proxy call fails
	_, err = c.Ping(background, &access.PingRequest{})
	if err == nil {
		t.Fatal(fmt.Errorf("should fail on no connections"))
	}

	<-done
	<-done
}

func makeFlowLite(address string, done chan int) (net.Listener, error) {
	l, err := net.Listen("unix", address)
	if err != nil {
		return nil, err
	}

	go func(done chan int) {
		for {
			c, err := l.Accept()
			if err != nil {
				break
			}

			b := make([]byte, 3)
			_, _ = c.Read(b)
			_, _ = c.Write(b)
			_ = c.Close()
		}
		done <- 1
	}(done)
	return l, err
}

func callFlowLite(address string) error {
	c, err := net.Dial("unix", address)
	if err != nil {
		return err
	}
	o := []byte("abc")
	_, _ = c.Write(o)
	i := make([]byte, 3)
	_, _ = c.Read(i)
	if string(o) != string(i) {
		return fmt.Errorf("no match")
	}
	_ = c.Close()
	_ = MockFlowAccessAPI{}
	return err
}

func newFlowLite(network string, address string, done chan int) (*grpc.Server, *net.Listener, error) {
	l, err := net.Listen(network, address)
	if err != nil {
		return nil, nil, err
	}
	s := grpc.NewServer()
	go func(done chan int) {
		access.RegisterAccessAPIServer(s, MockFlowAccessAPI{})
		_ = s.Serve(l)
		done <- 1
	}(done)
	return s, &l, nil
}

func openFlowLite(address string) error {
	c, err := grpc.Dial(
		"unix://"+address,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
		grpc.WithInsecure())
	if err != nil {
		return err
	}
	a := access.NewAccessAPIClient(c)

	background := context.Background()

	_, err = a.Ping(background, &access.PingRequest{})
	if err != nil {
		return err
	}

	return nil
}

type MockFlowAccessAPI struct {
	access.AccessAPIServer
}

// Ping is used to check if the access node is alive and healthy.
func (p MockFlowAccessAPI) Ping(context.Context, *access.PingRequest) (*access.PingResponse, error) {
	return &access.PingResponse{}, nil
}

//// GetLatestBlockHeader gets the latest sealed or unsealed block header.
//GetLatestBlockHeader(context.Context, *GetLatestBlockHeaderRequest) (*BlockHeaderResponse, error)
//// GetBlockHeaderByID gets a block header by ID.
//GetBlockHeaderByID(context.Context, *GetBlockHeaderByIDRequest) (*BlockHeaderResponse, error)
//// GetBlockHeaderByHeight gets a block header by height.
//GetBlockHeaderByHeight(context.Context, *GetBlockHeaderByHeightRequest) (*BlockHeaderResponse, error)
//// GetLatestBlock gets the full payload of the latest sealed or unsealed
//// block.
//GetLatestBlock(context.Context, *GetLatestBlockRequest) (*BlockResponse, error)
//// GetBlockByID gets a full block by ID.
//GetBlockByID(context.Context, *GetBlockByIDRequest) (*BlockResponse, error)
//// GetBlockByHeight gets a full block by height.
//GetBlockByHeight(context.Context, *GetBlockByHeightRequest) (*BlockResponse, error)
//// GetCollectionByID gets a collection by ID.
//GetCollectionByID(context.Context, *GetCollectionByIDRequest) (*CollectionResponse, error)
//// SendTransaction submits a transaction to the network.
//SendTransaction(context.Context, *SendTransactionRequest) (*SendTransactionResponse, error)
//// GetTransaction gets a transaction by ID.
//GetTransaction(context.Context, *GetTransactionRequest) (*TransactionResponse, error)
//// GetTransactionResult gets the result of a transaction.
//GetTransactionResult(context.Context, *GetTransactionRequest) (*TransactionResultResponse, error)
//// GetTransactionResultByIndex gets the result of a transaction at a specified
//// block and index
//GetTransactionResultByIndex(context.Context, *GetTransactionByIndexRequest) (*TransactionResultResponse, error)
//// GetTransactionResultsByBlockID gets all the transaction results for a
//// specified block
//GetTransactionResultsByBlockID(context.Context, *GetTransactionsByBlockIDRequest) (*TransactionResultsResponse, error)
//// GetTransactionsByBlockID gets all the transactions for a specified block
//GetTransactionsByBlockID(context.Context, *GetTransactionsByBlockIDRequest) (*TransactionsResponse, error)
//// GetAccount is an alias for GetAccountAtLatestBlock.
////
//// Warning: this function is deprecated. It behaves identically to
//// GetAccountAtLatestBlock and will be removed in a future version.
//GetAccount(context.Context, *GetAccountRequest) (*GetAccountResponse, error)
//// GetAccountAtLatestBlock gets an account by address from the latest sealed
//// execution state.
//GetAccountAtLatestBlock(context.Context, *GetAccountAtLatestBlockRequest) (*AccountResponse, error)
//// GetAccountAtBlockHeight gets an account by address at the given block
//// height
//GetAccountAtBlockHeight(context.Context, *GetAccountAtBlockHeightRequest) (*AccountResponse, error)
//// ExecuteScriptAtLatestBlock executes a read-only Cadence script against the
//// latest sealed execution state.
//ExecuteScriptAtLatestBlock(context.Context, *ExecuteScriptAtLatestBlockRequest) (*ExecuteScriptResponse, error)
//// ExecuteScriptAtBlockID executes a ready-only Cadence script against the
//// execution state at the block with the given ID.
//ExecuteScriptAtBlockID(context.Context, *ExecuteScriptAtBlockIDRequest) (*ExecuteScriptResponse, error)
//// ExecuteScriptAtBlockHeight executes a ready-only Cadence script against the
//// execution state at the given block height.
//ExecuteScriptAtBlockHeight(context.Context, *ExecuteScriptAtBlockHeightRequest) (*ExecuteScriptResponse, error)
//// GetEventsForHeightRange retrieves events emitted within the specified block
//// range.
//GetEventsForHeightRange(context.Context, *GetEventsForHeightRangeRequest) (*EventsResponse, error)
//// GetEventsForBlockIDs retrieves events for the specified block IDs and event
//// type.
//GetEventsForBlockIDs(context.Context, *GetEventsForBlockIDsRequest) (*EventsResponse, error)
//// GetNetworkParameters retrieves the Flow network details
//GetNetworkParameters(context.Context, *GetNetworkParametersRequest) (*GetNetworkParametersResponse, error)
//// GetLatestProtocolStateSnapshot retrieves the latest sealed protocol state
//// snapshot. Used by Flow nodes joining the network to bootstrap a
//// space-efficient local state.
//GetLatestProtocolStateSnapshot(context.Context, *GetLatestProtocolStateSnapshotRequest) (*ProtocolStateSnapshotResponse, error)
//// GetExecutionResultForBlockID returns Execution Result for a given block.
//// At present, Access Node might not have execution results for every block
//// and as usual, until sealed, this data can change
//GetExecutionResultForBlockID(context.Context, *GetExecutionResultForBlockIDRequest) (*ExecutionResultForBlockIDResponse, error)
