package storage

import (
	"errors"
	"fmt"
	"math"
	"strings"

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

var ErrValidatorReqDataFormat error = errors.New("wrong input format: expected JSON")

type blocksRequest struct {
	requestType blocksRequestType
	value       interface{}
}

func parseN(m interface{}) (uint64, error) {
	n, ok := m.(float64)
	if !ok {
		return 0, fmt.Errorf("invalid value for \"n\": %v", n)
	}
	if math.Trunc(n) != n {
		return 0, fmt.Errorf("\"n\" must be an integer")
	}
	if n < 1 {
		return 0, fmt.Errorf("\"n\" must be at least 1")
	}
	return uint64(n), nil
}

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
