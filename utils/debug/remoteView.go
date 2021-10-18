package debug

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow/protobuf/go/flow/execution"
)

// RemoteView provides a view connected to a live execution node to read the registers
// writen values are kept inside a map
//
// TODO implement register touches
type RemoteView struct {
	Parent             *RemoteView
	Delta              map[string]flow.RegisterValue
	Cache              registerCache
	connection         *grpc.ClientConn
	blockID            []byte
	executionAPIclient execution.ExecutionAPIClient
}

// A RemoteViewOption sets a configuration parameter for the remote view
type RemoteViewOption func(view *RemoteView) *RemoteView

// WithFileCache sets the output path to store
// register values so can be fetched from a file cache
// it loads the values from the cache upon object construction
func WithCache(cache registerCache) RemoteViewOption {
	return func(view *RemoteView) *RemoteView {
		view.Cache = cache
		return view
	}
}

// WithBlockID sets the blockID for the remote view, if not used
// remote view will use the latest sealed block
func WithBlockID(blockID flow.Identifier) RemoteViewOption {
	return func(view *RemoteView) *RemoteView {
		view.blockID = blockID[:]
		return view
	}
}

func NewRemoteView(grpcAddress string, opts ...RemoteViewOption) *RemoteView {
	conn, err := grpc.Dial(grpcAddress, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}

	view := &RemoteView{
		connection:         conn,
		executionAPIclient: execution.NewExecutionAPIClient(conn),
		Delta:              make(map[string]flow.RegisterValue),
		Cache:              newMemRegisterCache(),
	}

	view.blockID, err = view.getLatestBlockID()
	if err != nil {
		panic(err)
	}

	for _, applyOption := range opts {
		view = applyOption(view)
	}
	return view
}

func (v *RemoteView) Done() {
	v.connection.Close()
}

func (v *RemoteView) getLatestBlockID() ([]byte, error) {
	req := &execution.GetLatestBlockHeaderRequest{
		IsSealed: true,
	}

	resp, err := v.executionAPIclient.GetLatestBlockHeader(context.Background(), req)
	if err != nil {
		return nil, err
	}

	return resp.Block.Id, nil
}

func (v *RemoteView) NewChild() state.View {
	return &RemoteView{
		Parent:             v,
		executionAPIclient: v.executionAPIclient,
		connection:         v.connection,
		Cache:              newMemRegisterCache(),
		Delta:              make(map[string][]byte),
	}
}

func (v *RemoteView) MergeView(o state.View) error {
	var other *RemoteView
	var ok bool
	if other, ok = o.(*RemoteView); !ok {
		return fmt.Errorf("can not merge: view type mismatch (given: %T, expected:RemoteView)", o)
	}

	for k, value := range other.Delta {
		v.Delta[k] = value
	}
	return nil
}

func (v *RemoteView) DropDelta() {
	v.Delta = make(map[string]flow.RegisterValue)
}

func (v *RemoteView) Set(owner, controller, key string, value flow.RegisterValue) error {
	v.Delta[owner+"~"+controller+"~"+key] = value
	return nil
}

func (v *RemoteView) Get(owner, controller, key string) (flow.RegisterValue, error) {

	// first check the delta
	value, found := v.Delta[owner+"~"+controller+"~"+key]
	if found {
		return value, nil
	}

	// then check the read cache
	value, found = v.Cache.Get(owner, controller, key)
	if found {
		return value, nil
	}

	// then call the parent (if exist)
	if v.Parent != nil {
		return v.Parent.Get(owner, controller, key)
	}

	// last use the grpc api the
	req := &execution.GetRegisterAtBlockIDRequest{
		BlockId:            []byte(v.blockID),
		RegisterOwner:      []byte(owner),
		RegisterController: []byte(controller),
		RegisterKey:        []byte(key),
	}

	// TODO use a proper context for timeouts
	resp, err := v.executionAPIclient.GetRegisterAtBlockID(context.Background(), req)
	if err != nil {
		return nil, err
	}

	v.Cache.Set(owner, controller, key, resp.Value)

	// append value to the file cache

	return resp.Value, nil
}

// returns all the registers that has been touched
func (v *RemoteView) AllRegisters() []flow.RegisterID {
	panic("Not implemented yet")
}

func (v *RemoteView) RegisterUpdates() ([]flow.RegisterID, []flow.RegisterValue) {
	panic("Not implemented yet")
}

func (v *RemoteView) Touch(owner, controller, key string) error {
	// no-op for now
	return nil
}

func (v *RemoteView) Delete(owner, controller, key string) error {
	v.Delta[owner+"~"+controller+"~"+key] = nil
	return nil
}
