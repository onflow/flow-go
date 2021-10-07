package consensus

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
)

type requestType int

const (
	ID requestType = iota
	Height
	Latest
	LatestSealed
)

type ReadProtocolStateBlocksCommandData struct {
	requestType      requestType
	blockID          flow.Identifier
	blockHeight      uint64
	numBlocksToQuery uint
}

var ReadProtocolStateBlocksCommand commands.AdminCommand = commands.AdminCommand{
	Handler: func(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
		// TODO
		return req.ValidatorData, nil
	},
	Validator: func(req *admin.CommandRequest) error {
		data := &ReadProtocolStateBlocksCommandData{}
		block, ok := req.Data["block"]
		if !ok {
			return errors.New("the \"block\" field is required")
		}
		switch block := block.(type) {
		case string:
			if block == "latest" {
				data.requestType = Latest
			} else if block == "latest_sealed" {
				data.requestType = LatestSealed
			} else {
				b, err := hex.DecodeString(block)
				if err != nil {
					return fmt.Errorf("could not parse block ID: %v", block)
				}
				data.requestType = ID
				data.blockID = flow.BytesToID(b)
			}
		case float64:
			if block < 0 {
				return fmt.Errorf("block height must not be negative")
			}
			data.requestType = Height
			data.blockHeight = uint64(block)
		default:
			return fmt.Errorf("invalid value for \"block\": %v", block)
		}

		if n, ok := req.Data["n"]; ok {
			n, ok := n.(float64)
			if !ok {
				return fmt.Errorf("invalid value for \"n\": %v", n)
			}
			if n < 0 {
				return fmt.Errorf("\"n\" must not be negative")
			}
			data.numBlocksToQuery = uint(n)
		} else {
			data.numBlocksToQuery = 1
		}

		req.ValidatorData = data

		return nil
	},
}
