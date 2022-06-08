package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

var _ commands.AdminCommand = (*ReadResultsCommand)(nil)

type readResultsRequestType int

const (
	readResultsRequestByID readResultsRequestType = iota
	readResultsRequestByBlock
)

type readResultsRequest struct {
	requestType       readResultsRequestType
	value             interface{}
	numResultsToQuery uint64
}

type ReadResultsCommand struct {
	state   protocol.State
	results storage.ExecutionResults
}

func (r *ReadResultsCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	data := req.ValidatorData.(*readResultsRequest)
	var results []*flow.ExecutionResult
	var resultID flow.Identifier
	n := data.numResultsToQuery

	switch data.requestType {
	case readResultsRequestByID:
		resultID = data.value.(flow.Identifier)
	case readResultsRequestByBlock:
		if header, err := getBlockHeader(r.state, data.value.(*blocksRequest)); err != nil {
			return nil, fmt.Errorf("failed to get block header: %w", err)
		} else if result, err := r.results.ByBlockID(header.ID()); err != nil {
			return nil, fmt.Errorf("failed to get result by block ID: %w", err)
		} else {
			results = append(results, result)
			resultID = result.PreviousResultID
			n -= 1
		}
	}

	for i := uint64(0); i < n && resultID != flow.ZeroID; i++ {
		result, err := r.results.ByID(resultID)
		if err != nil {
			return nil, fmt.Errorf("failed to get result by ID: %w", err)
		}
		results = append(results, result)
		resultID = result.PreviousResultID
	}

	return commands.ConvertToInterfaceList(results)
}

func (r *ReadResultsCommand) Validator(req *admin.CommandRequest) error {
	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return ErrValidatorReqDataFormat
	}

	data := &readResultsRequest{}

	if result, ok := input["result"]; ok {
		errInvalidResultValue := fmt.Errorf("invalid value for \"result\": expected a result ID represented as a 64 character long hex string, but got: %v", result)
		result, ok := result.(string)
		if !ok {
			return errInvalidResultValue
		}
		resultID, err := flow.HexStringToIdentifier(result)
		if err != nil {
			return errInvalidResultValue
		}
		data.requestType = readResultsRequestByID
		data.value = resultID
	} else if block, ok := input["block"]; ok {
		br, err := parseBlocksRequest(block)
		if err != nil {
			return err
		}
		data.requestType = readResultsRequestByBlock
		data.value = br
	} else {
		return errors.New("either \"block\" or \"result\" field is required")
	}

	if n, ok := input["n"]; ok {
		if n, err := parseN(n); err != nil {
			return err
		} else {
			data.numResultsToQuery = n
		}
	} else {
		data.numResultsToQuery = 1
	}

	req.ValidatorData = data

	return nil
}

func NewReadResultsCommand(state protocol.State, storage storage.ExecutionResults) commands.AdminCommand {
	return &ReadResultsCommand{
		state,
		storage,
	}
}
