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

var _ commands.AdminCommand = (*ReadBlocksCommand)(nil)

type readBlocksRequest struct {
	blocksRequest    *blocksRequest
	numBlocksToQuery uint64
}

type ReadBlocksCommand struct {
	state  protocol.State
	blocks storage.Blocks
}

func (r *ReadBlocksCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	data := req.ValidatorData.(*readBlocksRequest)
	var result []*flow.Block
	var block *flow.Block

	if header, err := getBlockHeader(r.state, data.blocksRequest); err != nil {
		return nil, fmt.Errorf("failed to get block header: %w", err)
	} else if block, err = r.blocks.ByID(header.ID()); err != nil {
		return nil, fmt.Errorf("failed to get block by ID: %w", err)
	}

	result = append(result, block)
	firstHeight := block.Header.Height

	for i := uint64(1); i <= firstHeight && i < data.numBlocksToQuery; i++ {
		block, err := r.blocks.ByID(block.Header.ParentID)
		if err != nil {
			return nil, err
		}
		result = append(result, block)
	}

	return convertToInterfaceList(result)
}

func (r *ReadBlocksCommand) Validator(req *admin.CommandRequest) error {
	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return errors.New("wrong input format: expected JSON")
	}

	block, ok := input["block"]
	if !ok {
		return errors.New("the \"block\" field is required")
	}

	data := &readBlocksRequest{}
	if blocksRequest, err := parseBlocksRequest(block); err != nil {
		return err
	} else {
		data.blocksRequest = blocksRequest
	}

	if n, ok := input["n"]; ok {
		if n, err := parseN(n); err != nil {
			return err
		} else {
			data.numBlocksToQuery = n
		}
	} else {
		data.numBlocksToQuery = 1
	}

	req.ValidatorData = data

	return nil

}

func NewReadBlocksCommand(state protocol.State, storage storage.Blocks) commands.AdminCommand {
	return &ReadBlocksCommand{
		state,
		storage,
	}
}
