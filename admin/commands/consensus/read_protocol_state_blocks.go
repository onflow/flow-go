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
	numBlocksToQuery int
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
		switch block.(type) {
		case string:
			if block == "latest" {
				data.requestType = Latest
			} else if block == "latest_sealed" {
				data.requestType = LatestSealed
			} else {
				id := block.(string)
				b, err := hex.DecodeString(id)
				if err != nil {
					return fmt.Errorf("could not parse block ID: %v", id)
				}
				data.requestType = ID
				data.blockID = flow.BytesToID(b)
			}
		case float64:
			height := uint64(block.(float64))
			if height < 0 {
				return fmt.Errorf("block height must not be negative")
			}
			data.requestType = Height
			data.blockHeight = uint64(height)
		default:
			return fmt.Errorf("invalid value for \"block\": %v", block)
		}

		if n, ok := req.Data["n"]; ok {
			n, ok := n.(float64)
			if !ok {
				return fmt.Errorf("invalid value for \"n\": %v", n)
			}
			data.numBlocksToQuery = int(n)
		}

		req.ValidatorData = data

		return nil
	},
}
