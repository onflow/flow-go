package topicvalidator

import (
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/integration/tests/bft"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	// numOfAuthorizedEvents the number of authorized events that will be created when the test orchestrator is initialized.
	// The numOfAuthorizedEvents allows us to wait for a certain number of authorized messages to be received, this should
	// give the network enough time to process the unauthorized messages. This ensures us that the unauthorized messages
	// were indeed dropped and not unprocessed.
	// This threshold must be set to a low value to make the test conclude faster
	// by waiting for fewer events, which is beneficial when running the test
	// on an asynchronous network where event delivery can be unpredictable.
	numOfAuthorizedEvents = 5

	// numOfUnauthorizedEvents the number of unauthorized events per type to send by the test orchestrator.
	// This threshold must be set to a low value to make the test conclude faster
	// by waiting for fewer events, which is beneficial when running the test
	// on an asynchronous network where event delivery can be unpredictable.
	numOfUnauthorizedEvents = 5
)

// Orchestrator represents an insecure.AttackOrchestrator track incoming unauthorized messages and authorized messages received by victim nodes.
type Orchestrator struct {
	*bft.BaseOrchestrator
	unauthorizedEventsReceived []flow.Identifier
	authorizedEventsReceived   []flow.Identifier
	unauthorizedEvents         map[flow.Identifier]*insecure.EgressEvent
	authorizedEvents           map[flow.Identifier]*insecure.EgressEvent
	authorizedEventReceivedWg  sync.WaitGroup
	attackerAN                 flow.Identifier
	attackerEN                 flow.Identifier
	victimEN                   flow.Identifier
	victimVN                   flow.Identifier
}

var _ insecure.AttackOrchestrator = &Orchestrator{}

func NewOrchestrator(t *testing.T, logger zerolog.Logger, attackerAN, attackerEN, victimEN, victimVN flow.Identifier) *Orchestrator {
	orchestrator := &Orchestrator{
		BaseOrchestrator: &bft.BaseOrchestrator{
			T:      t,
			Logger: logger.With().Str("component", "bft-test-orchestrator").Logger(),
		},
		unauthorizedEventsReceived: make([]flow.Identifier, 0),
		authorizedEventsReceived:   make([]flow.Identifier, 0),
		unauthorizedEvents:         make(map[flow.Identifier]*insecure.EgressEvent),
		authorizedEvents:           make(map[flow.Identifier]*insecure.EgressEvent),
		authorizedEventReceivedWg:  sync.WaitGroup{},
		attackerAN:                 attackerAN,
		attackerEN:                 attackerEN,
		victimEN:                   victimEN,
		victimVN:                   victimVN,
	}

	orchestrator.initUnauthorizedEvents()
	orchestrator.initAuthorizedEvents()
	orchestrator.OnIngressEvent = append(orchestrator.OnIngressEvent, orchestrator.trackIngressEvents)
	return orchestrator
}

// trackIngressEvents callback that will track any unauthorized messages that are expected to be blocked at the topic validator.
// It also tracks all the authorized messages that are expected to be delivered to the node.
func (o *Orchestrator) trackIngressEvents(event *insecure.IngressEvent) error {
	// Track any unauthorized events that are received by corrupted nodes.
	// These events are unauthorized combinations of messages & channels and should be
	// dropped at the topic validator level.
	if _, ok := o.unauthorizedEvents[event.FlowProtocolEventID]; ok {
		o.unauthorizedEventsReceived = append(o.unauthorizedEventsReceived, event.FlowProtocolEventID)
		o.Logger.Warn().Str("event_id", event.FlowProtocolEventID.String()).Msg("unauthorized ingress event received")
	}

	// track all authorized events sent during test
	if _, ok := o.authorizedEvents[event.FlowProtocolEventID]; ok {
		o.authorizedEventsReceived = append(o.authorizedEventsReceived, event.FlowProtocolEventID)
		o.authorizedEventReceivedWg.Done()
	}
	return nil
}

// sendUnauthorizedMsgs publishes a few combinations of unauthorized messages from the corrupted AN to the corrupted EN.
func (o *Orchestrator) sendUnauthorizedMsgs(t *testing.T) {
	for _, event := range o.unauthorizedEvents {
		err := o.OrchestratorNetwork.SendEgress(event)
		require.NoError(t, err)
	}
}

// sendAuthorizedMsgs sends a number of authorized messages.
func (o *Orchestrator) sendAuthorizedMsgs(t *testing.T) {
	for _, event := range o.authorizedEvents {
		err := o.OrchestratorNetwork.SendEgress(event)
		require.NoError(t, err)
	}
}

// initUnauthorizedEvents returns combinations of unauthorized messages and channels.
func (o *Orchestrator) initUnauthorizedEvents() {
	// message sent by unauthorized sender, AN is not authorized to publish block proposals
	o.initUnauthorizedMsgByRoleEvents(numOfUnauthorizedEvents)

	// message sent on unauthorized channel, AN is not authorized send sync request on consensus committee channel
	o.initUnauthorizedMsgOnChannelEvents(numOfUnauthorizedEvents)

	// message is not authorized to be sent via insecure.Protocol_UNICAST
	// unicast stream handler is expected to drop this message
	o.initUnauthorizedUnicastOnChannelEvents(numOfUnauthorizedEvents)

	// message is not authorized to be sent via insecure.Protocol_PUBLISH
	o.initUnauthorizedPublishOnChannelEvents(numOfUnauthorizedEvents)
}

// initAuthorizedEvents returns combinations of unauthorized messages and channels.
func (o *Orchestrator) initAuthorizedEvents() {
	channel := channels.RequestChunks
	for i := uint64(0); i < numOfAuthorizedEvents; i++ {
		chunkDataReq := &messages.ChunkDataRequest{
			ChunkID: unittest.IdentifierFixture(),
			Nonce:   rand.Uint64(),
		}
		eventID := unittest.GetFlowProtocolEventID(o.T, channel, chunkDataReq)
		event := &insecure.EgressEvent{
			CorruptOriginId:     o.victimVN,
			Channel:             channel,
			Protocol:            insecure.Protocol_PUBLISH,
			TargetNum:           0,
			TargetIds:           flow.IdentifierList{o.attackerEN},
			FlowProtocolEvent:   chunkDataReq,
			FlowProtocolEventID: eventID,
		}
		o.authorizedEvents[eventID] = event
		o.authorizedEventReceivedWg.Add(1)
	}
}

// initUnauthorizedMsgByRoleEvents sets n number of events where the sender is unauthorized to
// send the FlowProtocolEvent. In this case AN's are not authorized to send block proposals.
func (o *Orchestrator) initUnauthorizedMsgByRoleEvents(n int) {
	channel := channels.SyncCommittee
	for i := 0; i < n; i++ {
		unauthorizedProposal := unittest.ProposalFixture()
		eventID := unittest.GetFlowProtocolEventID(o.T, channel, unauthorizedProposal)
		unauthorizedMsgByRole := &insecure.EgressEvent{
			CorruptOriginId:     o.attackerAN,
			Channel:             channel,
			Protocol:            insecure.Protocol_PUBLISH,
			TargetNum:           0,
			TargetIds:           flow.IdentifierList{o.victimEN},
			FlowProtocolEvent:   unauthorizedProposal,
			FlowProtocolEventID: eventID,
		}
		o.unauthorizedEvents[eventID] = unauthorizedMsgByRole
	}
}

// initUnauthorizedMsgOnChannelEvents sets n number of events where the message is not
// authorized to be sent on the event channel.
func (o *Orchestrator) initUnauthorizedMsgOnChannelEvents(n int) {
	channel := channels.PushReceipts
	for i := 0; i < n; i++ {
		syncReq := &messages.SyncRequest{
			Nonce:  rand.Uint64(),
			Height: rand.Uint64(),
		}
		eventID := unittest.GetFlowProtocolEventID(o.T, channel, syncReq)
		unauthorizedMsgOnChannel := &insecure.EgressEvent{
			CorruptOriginId:     o.attackerAN,
			Channel:             channel,
			Protocol:            insecure.Protocol_PUBLISH,
			TargetNum:           0,
			TargetIds:           flow.IdentifierList{o.victimEN},
			FlowProtocolEvent:   syncReq,
			FlowProtocolEventID: eventID,
		}
		o.unauthorizedEvents[eventID] = unauthorizedMsgOnChannel
	}
}

// initUnauthorizedUnicastOnChannelEvents sets n number of events where the message is not
// authorized to be sent via insecure.Protocol_UNICAST on the event channel.
func (o *Orchestrator) initUnauthorizedUnicastOnChannelEvents(n int) {
	channel := channels.SyncCommittee
	for i := 0; i < n; i++ {
		syncReq := &messages.SyncRequest{
			Nonce:  rand.Uint64(),
			Height: rand.Uint64(),
		}
		eventID := unittest.GetFlowProtocolEventID(o.T, channel, syncReq)
		unauthorizedUnicastOnChannel := &insecure.EgressEvent{
			CorruptOriginId:     o.attackerAN,
			Channel:             channel,
			Protocol:            insecure.Protocol_UNICAST,
			TargetNum:           0,
			TargetIds:           flow.IdentifierList{o.victimEN},
			FlowProtocolEvent:   syncReq,
			FlowProtocolEventID: eventID,
		}
		o.unauthorizedEvents[eventID] = unauthorizedUnicastOnChannel
	}
}

// initUnauthorizedPublishOnChannelEvents sets n number of events where the message is not
// authorized to be sent via insecure.Protocol_PUBLISH on the event channel.
func (o *Orchestrator) initUnauthorizedPublishOnChannelEvents(n int) {
	channel := channels.ProvideChunks
	for i := 0; i < n; i++ {
		chunkDataResponse := unittest.ChunkDataResponseMsgFixture(unittest.IdentifierFixture())
		eventID := unittest.GetFlowProtocolEventID(o.T, channel, chunkDataResponse)
		unauthorizedPublishOnChannel := &insecure.EgressEvent{
			CorruptOriginId:     o.attackerEN,
			Channel:             channel,
			Protocol:            insecure.Protocol_PUBLISH,
			TargetNum:           0,
			TargetIds:           flow.IdentifierList{o.victimVN},
			FlowProtocolEvent:   chunkDataResponse,
			FlowProtocolEventID: eventID,
		}
		o.unauthorizedEvents[eventID] = unauthorizedPublishOnChannel
	}
}
