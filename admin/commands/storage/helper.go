package storage

import (
	"fmt"
	"math"
	"strings"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

type blocksRequestType int

const (
	blocksRequestByID blocksRequestType = iota
	blocksRequestByHeight
	blocksRequestFinal
	blocksRequestSealed
)

const (
	FINAL  = "final"
	SEALED = "sealed"
)

type blocksRequest struct {
	requestType blocksRequestType
	value       interface{}
}

// parseN verifies that the input is an integral float64 value >=1.
// All generic errors indicate a benign validation failure, and should be wrapped by the caller.
func parseN(m interface{}) (uint64, error) {
	n, ok := m.(float64)
	if !ok {
		return 0, fmt.Errorf("invalid value for \"n\": %v", n)
	}
	if math.Trunc(n) != n {
		return 0, fmt.Errorf("\"n\" must be an integer, got: %v", n)
	}
	if n < 1 {
		return 0, fmt.Errorf("\"n\" must be at least 1, got: %v", n)
	}
	return uint64(n), nil
}

// parseBlocksRequest parses the block field of an admin request.
// All generic errors indicate a benign validation failure, and should be wrapped by the caller.
func parseBlocksRequest(block interface{}) (*blocksRequest, error) {
	errInvalidBlockValue := fmt.Errorf("invalid value for \"block\": expected %q, %q, block ID, or block height, but got: %v", FINAL, SEALED, block)
	req := &blocksRequest{}

	switch block := block.(type) {
	case string:
		block = strings.ToLower(strings.TrimSpace(block))
		if block == FINAL {
			req.requestType = blocksRequestFinal
		} else if block == SEALED {
			req.requestType = blocksRequestSealed
		} else if id, err := flow.HexStringToIdentifier(block); err == nil {
			req.requestType = blocksRequestByID
			req.value = id
		} else {
			return nil, errInvalidBlockValue
		}
	case float64:
		if block < 0 || math.Trunc(block) != block {
			return nil, errInvalidBlockValue
		}
		req.requestType = blocksRequestByHeight
		req.value = uint64(block)
	default:
		return nil, errInvalidBlockValue
	}

	return req, nil
}

func getBlockHeader(state protocol.State, req *blocksRequest) (*flow.Header, error) {
	switch req.requestType {
	case blocksRequestByID:
		return state.AtBlockID(req.value.(flow.Identifier)).Head()
	case blocksRequestByHeight:
		return state.AtHeight(req.value.(uint64)).Head()
	case blocksRequestFinal:
		return state.Final().Head()
	case blocksRequestSealed:
		return state.Sealed().Head()
	default:
		return nil, fmt.Errorf("invalid request type: %v", req.requestType)
	}
}

func parseHeightRangeRequestData(req *admin.CommandRequest) (*heightRangeReqData, error) {
	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return nil, admin.NewInvalidAdminReqFormatError("missing 'data' field")
	}

	startHeight, err := findUint64(input, "start-height")
	if err != nil {
		return nil, fmt.Errorf("invalid start-height: %w", err)
	}

	endHeight, err := findUint64(input, "end-height")
	if err != nil {
		return nil, fmt.Errorf("invalid end-height: %w", err)
	}

	if endHeight < startHeight {
		return nil, admin.NewInvalidAdminReqErrorf("end-height %v should not be smaller than start-height %v", endHeight, startHeight)
	}

	if endHeight-startHeight+1 > Max_Height_Range {
		return nil, admin.NewInvalidAdminReqErrorf("getting transactions for more than %v blocks at a time might have an impact to node's performance and is not allowed", Max_Height_Range)
	}

	return &heightRangeReqData{
		startHeight: startHeight,
		endHeight:   endHeight,
	}, nil
}

func parseString(req *admin.CommandRequest, field string) (string, error) {
	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return "", admin.NewInvalidAdminReqFormatError("missing 'data' field")
	}
	fieldValue, err := findString(input, field)
	if err != nil {
		return "", admin.NewInvalidAdminReqErrorf("missing %v field", field)
	}
	return fieldValue, nil
}
