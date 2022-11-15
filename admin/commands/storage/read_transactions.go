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
var MAX_HEIGHT_RANGE = uint64(1000)

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

	log.Info().Str("module", "admin-tool").Msgf("get transactions for height range [%v, %v]",
		data.startHeight, data.endHeight)
	blocks, err := finder.GetByHeightRange(data.startHeight, data.endHeight)
	if err != nil {
		return nil, err
	}

	return commands.ConvertToInterfaceList(blocks)
}

// Returns admin.InvalidAdminReqError for invalid inputs
func findUint64(input map[string]interface{}, field string) (uint64, error) {
	data, ok := input[field]
	if !ok {
		return 0, admin.NewInvalidAdminReqErrorf("missing required field '%s'", field)
	}
	val, err := parseN(data)
	if err != nil {
		return 0, admin.NewInvalidAdminReqErrorf("invalid 'n' field: %w", err)
	}

	return uint64(val), nil
}

// Validator validates the request.
// Returns admin.InvalidAdminReqError for invalid/malformed requests.
func (c *GetTransactionsCommand) Validator(req *admin.CommandRequest) error {
	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return admin.NewInvalidAdminReqFormatError("expected map[string]any")
	}

	startHeight, err := findUint64(input, "start-height")
	if err != nil {
		return err
	}

	endHeight, err := findUint64(input, "end-height")
	if err != nil {
		return err
	}

	if endHeight < startHeight {
		return admin.NewInvalidAdminReqErrorf("endHeight %v should not be smaller than startHeight %v", endHeight, startHeight)
	}

	if endHeight-startHeight+1 > MAX_HEIGHT_RANGE {
		return admin.NewInvalidAdminReqErrorf("getting transactions for more than %v blocks at a time might have an impact to node's performance and is not allowed", MAX_HEIGHT_RANGE)
	}

	req.ValidatorData = &getTransactionsReqData{
		startHeight: startHeight,
		endHeight:   endHeight,
	}

	return nil
}
