package debug

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/execution"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

// RemoteView provides a view connected to a live execution node to read the registers
// writen values are kept inside a map
//
// TODO implement register touches
type RemoteView struct {
	Parent             *RemoteView
	Delta              map[flow.RegisterID]flow.RegisterValue
	Cache              registerCache
	BlockID            []byte
	BlockHeader        *flow.Header
	connection         *grpc.ClientConn
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
		view.BlockID = blockID[:]
		var err error
		view.BlockHeader, err = view.getBlockHeader(blockID)
		if err != nil {
			panic(err)
		}
		return view
	}
}

func NewRemoteView(grpcAddress string, opts ...RemoteViewOption) *RemoteView {
	conn, err := grpc.Dial(grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	view := &RemoteView{
		connection:         conn,
		executionAPIclient: execution.NewExecutionAPIClient(conn),
		Delta:              make(map[flow.RegisterID]flow.RegisterValue),
		Cache:              newMemRegisterCache(),
	}

	view.BlockID, view.BlockHeader, err = view.getLatestBlockID()
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

func (v *RemoteView) getLatestBlockID() ([]byte, *flow.Header, error) {
	req := &execution.GetLatestBlockHeaderRequest{
		IsSealed: true,
	}

	resp, err := v.executionAPIclient.GetLatestBlockHeader(context.Background(), req)
	if err != nil {
		return nil, nil, err
	}

	// TODO set chainID and parentID
	header := &flow.Header{
		Height:    resp.Block.Height,
		Timestamp: resp.Block.Timestamp.AsTime(),
	}

	return resp.Block.Id, header, nil
}

func (v *RemoteView) getBlockHeader(blockID flow.Identifier) (*flow.Header, error) {
	req := &execution.GetBlockHeaderByIDRequest{
		Id: blockID[:],
	}

	resp, err := v.executionAPIclient.GetBlockHeaderByID(context.Background(), req)
	if err != nil {
		return nil, err
	}

	// TODO set chainID and parentID
	header := &flow.Header{
		Height:    resp.Block.Height,
		Timestamp: resp.Block.Timestamp.AsTime(),
	}

	return header, nil
}

func (v *RemoteView) NewChild() state.View {
	return &RemoteView{
		Parent:             v,
		executionAPIclient: v.executionAPIclient,
		connection:         v.connection,
		Cache:              newMemRegisterCache(),
		Delta:              make(map[flow.RegisterID][]byte),
	}
}

func (v *RemoteView) Merge(other state.ExecutionSnapshot) error {
	for _, entry := range other.UpdatedRegisters() {
		v.Delta[entry.Key] = entry.Value
	}
	return nil
}

func (v *RemoteView) SpockSecret() []byte {
	return nil
}

func (v *RemoteView) Meter() *meter.Meter {
	return nil
}

func (v *RemoteView) DropChanges() error {
	v.Delta = make(map[flow.RegisterID]flow.RegisterValue)
	return nil
}

func (v *RemoteView) Set(id flow.RegisterID, value flow.RegisterValue) error {
	v.Delta[id] = value
	return nil
}

func (v *RemoteView) Get(id flow.RegisterID) (flow.RegisterValue, error) {

	// first check the delta
	value, found := v.Delta[id]
	if found {
		return value, nil
	}

	// then check the read cache
	value, found = v.Cache.Get(id.Owner, id.Key)
	if found {
		return value, nil
	}

	// then call the parent (if exist)
	if v.Parent != nil {
		return v.Parent.Get(id)
	}

	// last use the grpc api the
	req := &execution.GetRegisterAtBlockIDRequest{
		BlockId:       []byte(v.BlockID),
		RegisterOwner: []byte(id.Owner),
		RegisterKey:   []byte(id.Key),
	}

	// TODO use a proper context for timeouts
	resp, err := v.executionAPIclient.GetRegisterAtBlockID(context.Background(), req)
	if err != nil {
		return nil, err
	}

	v.Cache.Set(id.Owner, id.Key, resp.Value)

	// append value to the file cache

	return resp.Value, nil
}

// returns all the register ids that has been updated
func (v *RemoteView) UpdatedRegisterIDs() []flow.RegisterID {
	panic("Not implemented yet")
}

// returns all the register ids that has been touched
func (v *RemoteView) AllRegisterIDs() []flow.RegisterID {
	panic("Not implemented yet")
}

func (v *RemoteView) UpdatedRegisters() flow.RegisterEntries {
	panic("Not implemented yet")
}
