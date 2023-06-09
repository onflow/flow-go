package storage

import (
	"context"
	"strings"

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

func findString(input map[string]interface{}, field string) (string, error) {
	data, ok := input[field]
	if !ok {
		return "", admin.NewInvalidAdminReqErrorf("missing required field '%s'", field)
	}

	str, ok := data.(string)
	if !ok {
		return "", admin.NewInvalidAdminReqErrorf("field '%s' is not string", field)
	}

	return strings.ToLower(strings.TrimSpace(str)), nil
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
