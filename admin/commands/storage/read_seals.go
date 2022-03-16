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

var _ commands.AdminCommand = (*ReadSealsCommand)(nil)

type readSealsRequestType int

const (
	readSealsRequestByID readSealsRequestType = iota
	readSealsRequestByBlock
)

type readSealsRequest struct {
	requestType      readSealsRequestType
	value            interface{}
	numBlocksToQuery uint64
}

type sealInfo struct {
	BlockID     flow.Identifier
	BlockHeight uint64
	SealID      flow.Identifier
}

type blockSeals struct {
	BlockID     flow.Identifier
	BlockHeight uint64
	Seals       []*flow.Seal
	LastSeal    *sealInfo
}

type ReadSealsCommand struct {
	state protocol.State
	seals storage.Seals
	index storage.Index
}

func (r *ReadSealsCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	data := req.ValidatorData.(*readSealsRequest)

	if data.requestType == readSealsRequestByID {
		if seal, err := r.seals.ByID(data.value.(flow.Identifier)); err != nil {
			return nil, fmt.Errorf("failed to get seal by ID: %w", err)
		} else {
			return commands.ConvertToMap(seal)
		}
	}

	var result []*blockSeals
	var br *blocksRequest = data.value.(*blocksRequest)

	for i := uint64(0); i < data.numBlocksToQuery; i++ {
		header, err := getBlockHeader(r.state, br)
		if err != nil {
			return nil, fmt.Errorf("failed to get block header: %w", err)
		}
		blockID := header.ID()

		index, err := r.index.ByBlockID(blockID)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve index for block %#v: %w", blockID, err)
		}
		lastSeal, err := r.seals.ByBlockID(blockID)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve last seal for block %#v: %w", blockID, err)
		}
		lastSealed, err := r.state.AtBlockID(lastSeal.BlockID).Head()
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve header for last sealed block at block %#v: %w", blockID, err)
		}
		bs := &blockSeals{
			BlockID:     blockID,
			BlockHeight: header.Height,
			LastSeal: &sealInfo{
				BlockID:     lastSeal.BlockID,
				BlockHeight: lastSealed.Height,
				SealID:      lastSeal.ID(),
			},
		}

		seals := make(map[uint64]*flow.Seal)
		for _, sealID := range index.SealIDs {
			seal, err := r.seals.ByID(sealID)
			if err != nil {
				return nil, fmt.Errorf("failed to retrieve seal with ID %#v: %w", sealID, err)
			}
			h, err := r.state.AtBlockID(seal.BlockID).Head()
			if err != nil {
				return nil, fmt.Errorf("failed to retrieve header for sealed block %#v: %w", seal.BlockID, err)
			}
			seals[h.Height] = seal
		}

		bs.Seals = make([]*flow.Seal, len(index.SealIDs))
		for i := uint64(0); i < uint64(len(seals)); i++ {
			bs.Seals[i] = seals[lastSealed.Height-i]
		}

		result = append(result, bs)

		if header.Height == 0 {
			break
		}

		br = &blocksRequest{
			blocksRequestByID,
			header.ParentID,
		}
	}

	return commands.ConvertToInterfaceList(result)
}

func (r *ReadSealsCommand) Validator(req *admin.CommandRequest) error {
	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return ErrValidatorReqDataFormat
	}

	data := &readSealsRequest{}

	if seal, ok := input["seal"]; ok {
		errInvalidResultValue := fmt.Errorf("invalid value for \"seal\": expected a seal ID represented as a 64 character long hex string, but got: %v", seal)
		seal, ok := seal.(string)
		if !ok {
			return errInvalidResultValue
		}
		sealID, err := flow.HexStringToIdentifier(seal)
		if err != nil {
			return errInvalidResultValue
		}
		data.requestType = readSealsRequestByID
		data.value = sealID
	} else if block, ok := input["block"]; ok {
		br, err := parseBlocksRequest(block)
		if err != nil {
			return err
		}
		data.requestType = readSealsRequestByBlock
		data.value = br
		if n, ok := input["n"]; ok {
			if n, err := parseN(n); err != nil {
				return err
			} else {
				data.numBlocksToQuery = n
			}
		} else {
			data.numBlocksToQuery = 1
		}
	} else {
		return errors.New("either \"block\" or \"seal\" field is required")
	}

	req.ValidatorData = data

	return nil
}

func NewReadSealsCommand(state protocol.State, storage storage.Seals, index storage.Index) commands.AdminCommand {
	return &ReadSealsCommand{
		state,
		storage,
		index,
	}
}
