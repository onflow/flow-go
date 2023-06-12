package storage

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/cmd/util/cmd/export-json-transactions/transactions"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

var _ commands.AdminCommand = (*GetTransactionsCommand)(nil)

// max number of block height to query transactions from
var Max_Height_Range = uint64(1000)

type heightRangeReqData struct {
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
	data := req.ValidatorData.(*heightRangeReqData)

	finder := &transactions.Finder{
		State:       c.state,
		Payloads:    c.payloads,
		Collections: c.collections,
	}

	log.Info().Str("module", "admin-tool").Msgf("get transactions for height range [%v, %v]",
		data.startHeight, data.endHeight)
	blocks, err := finder.GetByHeightRange(data.startHeight, data.endHeight)
	if err != nil {
		return nil, err
	}

	return commands.ConvertToInterfaceList(blocks)
}

// Validator validates the request.
// Returns admin.InvalidAdminReqError for invalid/malformed requests.
func (c *GetTransactionsCommand) Validator(req *admin.CommandRequest) error {
	data, err := parseHeightRangeRequestData(req)
	if err != nil {
		return err
	}
	req.ValidatorData = data
	return nil
}
