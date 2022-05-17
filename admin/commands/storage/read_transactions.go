package storage

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/cmd/util/cmd/export-json-transactions/transactions"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

var _ commands.AdminCommand = (*GetTransactionsCommand)(nil)

type getTransactionsReqData struct {
	startHeight uint64
	endHeight   uint64
}

type GetTransactionsCommand struct {
	state       protocol.State
	payloads    storage.Payloads
	collections storage.Collections
}

func NewGetTransactionsCommand(state protocol.State, payloads storage.Payloads, collections storage.Collections) *GetTransactionsCommand {
	return &GetTransactionsCommand{
		state:       state,
		payloads:    payloads,
		collections: collections,
	}
}

func (c *GetTransactionsCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	data := req.ValidatorData.(*getTransactionsReqData)

	finder := &transactions.Finder{
		State:       c.state,
		Payloads:    c.payloads,
		Collections: c.collections,
	}

	blocks, err := finder.GetByHeightRange(data.startHeight, data.endHeight)
	if err != nil {
		return nil, err
	}

	return commands.ConvertToInterfaceList(blocks)
}

func usageErr(msg string) error {
	return fmt.Errorf("required flags \"start-height\", \"end-height\", %s", msg)
}

func findUint64(input map[string]interface{}, field string) (uint64, error) {
	data, ok := input[field]
	if !ok {
		return 0, usageErr(fmt.Sprintf("%s not set", field))
	}
	val, err := parseN(data)
	if err != nil {
		return 0, usageErr(fmt.Sprintf("%s must be a uint64 value, but got %v: %v", field, data, err))
	}

	return uint64(val), nil
}

func (c *GetTransactionsCommand) Validator(req *admin.CommandRequest) error {
	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return usageErr("invalid json")
	}

	startHeight, err := findUint64(input, "start-height")
	if err != nil {
		return err
	}

	endHeight, err := findUint64(input, "end-height")
	if err != nil {
		return err
	}

	if endHeight-startHeight > 99 {
		return fmt.Errorf("getting transactions for more than 100 blocks at a time might have an impact to node's performance")
	}

	req.ValidatorData = &getTransactionsReqData{
		startHeight: startHeight,
		endHeight:   endHeight,
	}

	return nil
}
