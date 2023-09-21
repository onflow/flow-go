package state_synchronization

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
)

var _ commands.AdminCommand = (*ReadRegisterDataCommand)(nil)

type registerRequest struct {
	registerID flow.RegisterID
	height     uint64
}

type ReadRegisterDataCommand struct {
	indexer *indexer.ExecutionState
}

// Handler returns hex encoded data.
func (r *ReadRegisterDataCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	data := req.ValidatorData.(*registerRequest)

	registers, err := r.indexer.RegisterValues(flow.RegisterIDs{data.registerID}, data.height)
	if err != nil {
		return nil, fmt.Errorf("failed to get register data: %w", err)
	}

	return fmt.Sprintf("%x", registers[0]), nil
}

// Validator validates the request.
// Returns admin.InvalidAdminReqError for invalid/malformed requests.
func (r *ReadRegisterDataCommand) Validator(req *admin.CommandRequest) error {
	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return admin.NewInvalidAdminReqFormatError("expected map[string]any")
	}

	ownerRaw, ok := input["owner"]
	if !ok {
		return admin.NewInvalidAdminReqErrorf("missing required field 'owner")
	}

	ownerHex, ok := ownerRaw.(string)
	if !ok {
		return admin.NewInvalidAdminReqParameterError("owner", "must be a string", ownerRaw)
	}

	keyRaw, ok := input["key"]
	if !ok {
		return admin.NewInvalidAdminReqErrorf("missing required field 'key")
	}

	key, ok := keyRaw.(string)
	if !ok {
		return admin.NewInvalidAdminReqParameterError("key", "must be a string", ownerRaw)
	}

	heightRaw, ok := input["height"]
	if !ok {
		return admin.NewInvalidAdminReqErrorf("missing required field 'height")
	}

	heightStr, ok := heightRaw.(string)
	if !ok {
		return admin.NewInvalidAdminReqParameterError("key", "must be a string", heightRaw)
	}

	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		return admin.NewInvalidAdminReqParameterError("height", "must be a number", err)
	}

	owner, err := hex.DecodeString(ownerHex)
	if err != nil {
		return admin.NewInvalidAdminReqParameterError("owner", "must be a hex-encoded string", err)
	}

	req.ValidatorData = &registerRequest{
		registerID: flow.NewRegisterID(string(owner), key),
		height:     height,
	}

	return nil
}

func NewRegisterDataCommand(indexer *indexer.ExecutionState) commands.AdminCommand {
	return &ReadRegisterDataCommand{
		indexer,
	}
}
