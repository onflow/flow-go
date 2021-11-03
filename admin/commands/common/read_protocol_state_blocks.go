package common

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

var _ commands.AdminCommand = (*ReadProtocolStateBlocksCommand)(nil)

type requestType int

const (
	ID requestType = iota
	Height
	Final
	Sealed
)

type requestData struct {
	requestType      requestType
	blockID          flow.Identifier
	blockHeight      uint64
	numBlocksToQuery uint
}

type ReadProtocolStateBlocksCommand struct {
	state  protocol.State
	blocks storage.Blocks
}

func (r *ReadProtocolStateBlocksCommand) getBlockByHeight(height uint64) (*flow.Block, error) {
	header, err := r.state.AtHeight(height).Head()
	if err != nil {
		return nil, fmt.Errorf("could not get header by height: %v, %w", height, err)
	}

	block, err := r.getBlockByHeader(header)
	if err != nil {
		return nil, fmt.Errorf("could not get block by header: %w", err)
	}
	return block, nil
}

func (r *ReadProtocolStateBlocksCommand) getFinal() (*flow.Block, error) {
	header, err := r.state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized, %w", err)
	}

	block, err := r.getBlockByHeader(header)
	if err != nil {
		return nil, fmt.Errorf("could not get block by header: %w", err)
	}
	return block, nil
}

func (r *ReadProtocolStateBlocksCommand) getSealed() (*flow.Block, error) {
	header, err := r.state.Sealed().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get sealed block, %w", err)
	}

	block, err := r.getBlockByHeader(header)
	if err != nil {
		return nil, fmt.Errorf("could not get block by header: %w", err)
	}
	return block, nil
}

func (r *ReadProtocolStateBlocksCommand) getBlockByID(blockID flow.Identifier) (*flow.Block, error) {
	header, err := r.state.AtBlockID(blockID).Head()
	if err != nil {
		return nil, fmt.Errorf("could not get header by blockID: %v, %w", blockID, err)
	}

	block, err := r.getBlockByHeader(header)
	if err != nil {
		return nil, fmt.Errorf("could not get block by header: %w", err)
	}
	return block, nil
}

func (r *ReadProtocolStateBlocksCommand) getBlockByHeader(header *flow.Header) (*flow.Block, error) {
	blockID := header.ID()
	block, err := r.blocks.ByID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get block by ID %v: %w", blockID, err)
	}
	return block, nil
}

func (r *ReadProtocolStateBlocksCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	data := req.ValidatorData.(*requestData)
	var result []*flow.Block
	var block *flow.Block
	var err error

	switch data.requestType {
	case ID:
		block, err = r.getBlockByID(data.blockID)
	case Height:
		block, err = r.getBlockByHeight(data.blockHeight)
	case Final:
		block, err = r.getFinal()
	case Sealed:
		block, err = r.getSealed()
	}

	if err != nil {
		return nil, err
	}

	result = append(result, block)
	firstHeight := int64(block.Header.Height)

	for height := firstHeight - 1; height >= 0 && height > firstHeight-int64(data.numBlocksToQuery); height-- {
		block, err = r.getBlockByHeight(uint64(height))
		if err != nil {
			return nil, err
		}
		result = append(result, block)
	}

	var resultList []interface{}
	bytes, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &resultList)

	return resultList, err
}

func (r *ReadProtocolStateBlocksCommand) Validator(req *admin.CommandRequest) error {
	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return errors.New("wrong input format")
	}

	block, ok := input["block"]
	if !ok {
		return errors.New("the \"block\" field is required")
	}

	errInvalidBlockValue := fmt.Errorf("invalid value for \"block\": %v", block)
	data := &requestData{}

	switch block := block.(type) {
	case string:
		block = strings.ToLower(strings.TrimSpace(block))
		if block == "final" {
			data.requestType = Final
		} else if block == "sealed" {
			data.requestType = Sealed
		} else if len(block) == 2*flow.IdentifierLen {
			b, err := hex.DecodeString(block)
			if err != nil {
				return errInvalidBlockValue
			}
			data.requestType = ID
			data.blockID = flow.HashToID(b)
		} else {
			return errInvalidBlockValue
		}
	case float64:
		if block < 0 || math.Trunc(block) != block {
			return errInvalidBlockValue
		}
		data.requestType = Height
		data.blockHeight = uint64(block)
	default:
		return errInvalidBlockValue
	}

	if n, ok := input["n"]; ok {
		n, ok := n.(float64)
		if !ok {
			return fmt.Errorf("invalid value for \"n\": %v", n)
		}
		if math.Trunc(n) != n {
			return fmt.Errorf("\"n\" must be an integer")
		}
		if n < 1 {
			return fmt.Errorf("\"n\" must be at least 1")
		}
		data.numBlocksToQuery = uint(n)
	} else {
		data.numBlocksToQuery = 1
	}

	req.ValidatorData = data

	return nil

}

func NewReadProtocolStateBlocksCommand(state protocol.State, storage storage.Blocks) commands.AdminCommand {
	return &ReadProtocolStateBlocksCommand{
		state,
		storage,
	}
}
