package common

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

var _ commands.AdminCommand = (*GetIdentityCommand)(nil)

type getIdentityRequestType int

const (
	FlowID getIdentityRequestType = iota
	PeerID
)

type getIdentityRequestData struct {
	requestType getIdentityRequestType
	flowID      flow.Identifier
	peerID      peer.ID
}

type GetIdentityCommand struct {
	idProvider module.IdentityProvider
}

func (r *GetIdentityCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	data := req.ValidatorData.(*getIdentityRequestData)

	if data.requestType == FlowID {
		identity, ok := r.idProvider.ByNodeID(data.flowID)
		if !ok {
			return nil, fmt.Errorf("no identity found for flow ID: %s", data.flowID)
		}
		return commands.ConvertToMap(identity)
	} else {
		identity, ok := r.idProvider.ByPeerID(data.peerID)
		if !ok {
			return nil, fmt.Errorf("no identity found for peer ID: %s", data.peerID)
		}
		return commands.ConvertToMap(identity)
	}
}

// Validator validates the request.
// Returns admin.InvalidAdminReqError for invalid/malformed requests.
func (r *GetIdentityCommand) Validator(req *admin.CommandRequest) error {
	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return admin.NewInvalidAdminReqFormatError("expected map[string]any")
	}

	data := &getIdentityRequestData{}

	req.ValidatorData = data

	if flowID, ok := input["flow_id"]; ok {
		data.requestType = FlowID

		if flowID, ok := flowID.(string); ok {
			if len(flowID) == 2*flow.IdentifierLen {
				if b, err := hex.DecodeString(flowID); err == nil {
					data.flowID = flow.HashToID(b)
					return nil
				}
			}
		}
		return admin.NewInvalidAdminReqParameterError("flow_id", "must be 64-char hex string", flowID)
	} else if peerID, ok := input["peer_id"]; ok {
		data.requestType = PeerID

		if peerID, ok := peerID.(string); ok {
			if pid, err := peer.Decode(peerID); err == nil {
				data.peerID = pid
				return nil
			}
		}
		return admin.NewInvalidAdminReqParameterError("peer_id", "must be valid peer id string", peerID)
	}
	return admin.NewInvalidAdminReqErrorf("either \"flow_id\" or \"peer_id\" field is required")
}

func NewGetIdentityCommand(idProvider module.IdentityProvider) commands.AdminCommand {
	return &GetIdentityCommand{
		idProvider: idProvider,
	}
}
