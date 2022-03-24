package common

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/id"
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
	idProvider id.IdentityProvider
}

func (r *GetIdentityCommand) Handler(ctx context.Context, req *admin.CommandRequest) (interface{}, error) {
	data := req.ValidatorData.(*getIdentityRequestData)

	if data.requestType == FlowID {
		identity, ok := r.idProvider.ByNodeID(data.flowID)
		if !ok {
			return nil, errors.New("no identity found for flow ID")
		}
		return commands.ConvertToMap(identity)
	} else {
		identity, ok := r.idProvider.ByPeerID(data.peerID)
		if !ok {
			return nil, errors.New("no identity found for peer ID")
		}
		return commands.ConvertToMap(identity)
	}
}

func (r *GetIdentityCommand) Validator(req *admin.CommandRequest) error {
	input, ok := req.Data.(map[string]interface{})
	if !ok {
		return errors.New("wrong input format")
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

		return fmt.Errorf("invalid value for \"flow_id\": %v", flowID)
	} else if peerID, ok := input["peer_id"]; ok {
		data.requestType = PeerID

		if peerID, ok := peerID.(string); ok {
			if pid, err := peer.Decode(peerID); err == nil {
				data.peerID = pid
				return nil
			}
		}

		return fmt.Errorf("invalid value for \"peer_id\": %v", peerID)
	} else {
		return errors.New("either \"flow_id\" or \"peer_id\" field is required")
	}
}

func NewGetIdentityCommand(idProvider id.IdentityProvider) commands.AdminCommand {
	return &GetIdentityCommand{
		idProvider: idProvider,
	}
}
