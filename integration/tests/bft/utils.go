package bft

import (
	"math/rand"
	"testing"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/unittest"
)

// RequestChunkDataPackEgressFixture returns an insecure.EgressEvent with messages.ChunkDataRequest payload and the provided node ID as the originID.
func RequestChunkDataPackEgressFixture(t *testing.T, originID, targetID flow.Identifier, protocol insecure.Protocol) *insecure.EgressEvent {
	channel := channels.RequestChunks
	chunkDataReq := &messages.ChunkDataRequest{
		ChunkID: unittest.IdentifierFixture(),
		Nonce:   rand.Uint64(),
	}
	eventID := unittest.GetFlowProtocolEventID(t, channel, chunkDataReq)
	return &insecure.EgressEvent{
		CorruptOriginId:     originID,
		Channel:             channel,
		Protocol:            protocol,
		TargetNum:           0,
		TargetIds:           flow.IdentifierList{targetID},
		FlowProtocolEvent:   chunkDataReq,
		FlowProtocolEventID: eventID,
	}
}
