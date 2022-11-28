package topicvalidator

import (
	"fmt"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	// numOfAuthorizedEvents the number of authorized events that will be created when the test orchestrator is initialized.
	// The numOfAuthorizedEvents allows us to wait for a certain number of authorized messages to be received, this should
	// give the network enough time to process the unauthorized messages. This ensures us that the unauthorized messages
	// were indeed dropped and not unprocessed.
	numOfAuthorizedEvents = 100

	// numOfUnauthorizedEvents the number of unauthorized events per type to send by the test orchestrator.
	numOfUnauthorizedEvents = 10
)

// testOrchestrator represents a simple testOrchestrator that passes through all incoming events.
type testOrchestrator struct {
	sync.Mutex
	t                          *testing.T
	logger                     zerolog.Logger
	orchestratorNetwork        insecure.OrchestratorNetwork
	codec                      network.Codec
	unauthorizedEventsReceived []string
	authorizedEventsReceived   []string
	unauthorizedEvents         map[string]*insecure.EgressEvent
	authorizedEvents           map[string]*insecure.EgressEvent
	authorizedEventsWg         sync.WaitGroup
	attackerAN                 flow.Identifier
	attackerEN                 flow.Identifier
	victimEN                   flow.Identifier
	victimVN                   flow.Identifier
}

var _ insecure.AttackOrchestrator = &testOrchestrator{}

func NewOrchestrator(t *testing.T, logger zerolog.Logger, attackerAN, attackerEN, victimEN, victimVN flow.Identifier) *testOrchestrator {
	orchestrator := &testOrchestrator{
		t:                          t,
		logger:                     logger.With().Str("component", "bft_test_orchestrator").Logger(),
		codec:                      unittest.NetworkCodec(),
		unauthorizedEventsReceived: make([]string, 0),
		authorizedEventsReceived:   make([]string, 0),
		unauthorizedEvents:         make(map[string]*insecure.EgressEvent),
		authorizedEvents:           make(map[string]*insecure.EgressEvent),
		authorizedEventsWg:         sync.WaitGroup{},
		attackerAN:                 attackerAN,
		attackerEN:                 attackerEN,
		victimEN:                   victimEN,
		victimVN:                   victimVN,
	}

	orchestrator.initUnauthorizedEvents()
	orchestrator.initAuthorizedEvents()

	return orchestrator
}

// HandleEgressEvent implements logic of processing the outgoing (egress) events received from a corrupted node.
func (o *testOrchestrator) HandleEgressEvent(event *insecure.EgressEvent) error {
	lg := o.logger.With().
		Hex("corrupt_origin_id", logging.ID(event.CorruptOriginId)).
		Str("channel", event.Channel.String()).
		Str("protocol", event.Protocol.String()).
		Uint32("target_num", event.TargetNum).
		Strs("target_ids", logging.IDs(event.TargetIds)).
		Str("flow_protocol_event", logging.Type(event.FlowProtocolEvent)).Logger()

	err := o.orchestratorNetwork.SendEgress(event)
	if err != nil {
		lg.Error().Err(err).Msg("could not pass through egress event")
		return err
	}

	lg.Info().Str("event_id", event.FlowProtocolEventID).Msg("egress event passed through successfully")
	return nil
}

// HandleIngressEvent implements logic of processing the incoming (ingress) events to a corrupt node.
// This handler will track any unauthorized messages that are expected to be blocked at the topic validator.
// It also tracks all the authorized messages that are expected to be delivered to the node.
func (o *testOrchestrator) HandleIngressEvent(event *insecure.IngressEvent) error {
	lg := o.logger.With().
		Hex("origin_id", logging.ID(event.OriginID)).
		Str("channel", event.Channel.String()).
		Str("corrupt_target_id", fmt.Sprintf("%v", event.CorruptTargetID)).
		Str("flow_protocol_event", fmt.Sprintf("%T", event.FlowProtocolEvent)).Logger()

	// Track any events unauthorized events that are received by corrupted nodes.
	// These events are unauthorized combinations of messages & channels and should be
	// dropped at the topic validator level.
	if _, ok := o.unauthorizedEvents[event.FlowProtocolEventID]; ok {
		o.unauthorizedEventsReceived = append(o.unauthorizedEventsReceived, event.FlowProtocolEventID)
		lg.Warn().Str("event_id", event.FlowProtocolEventID).Msg("unauthorized ingress event received")
	}

	// track all authorized events sent during test
	if _, ok := o.authorizedEvents[event.FlowProtocolEventID]; ok {
		o.authorizedEventsReceived = append(o.authorizedEventsReceived, event.FlowProtocolEventID)
		o.authorizedEventsWg.Done()
	}

	err := o.orchestratorNetwork.SendIngress(event)

	if err != nil {
		// since this is used for testing, if we encounter any RPC send error, crash the testOrchestrator.
		lg.Error().Err(err).Msg("could not pass through ingress event")
		return err
	}
	lg.Info().Str("event_id", event.FlowProtocolEventID).Msg("ingress event passed through successfully")
	return nil
}

func (o *testOrchestrator) Register(orchestratorNetwork insecure.OrchestratorNetwork) {
	o.orchestratorNetwork = orchestratorNetwork
}

// sendUnauthorizedMsgs publishes a few combinations of unauthorized messages from the corrupted AN to the corrupted EN.
func (o *testOrchestrator) sendUnauthorizedMsgs(t *testing.T) {
	for _, event := range o.unauthorizedEvents {
		err := o.orchestratorNetwork.SendEgress(event)
		require.NoError(t, err)
	}
}

// sendAuthorizedMsgs sends a number of authorized messages.
func (o *testOrchestrator) sendAuthorizedMsgs(t *testing.T) {
	for _, event := range o.authorizedEvents {
		err := o.orchestratorNetwork.SendEgress(event)
		require.NoError(t, err)
	}
}

// initUnauthorizedEvents returns combinations of unauthorized messages and channels.
func (o *testOrchestrator) initUnauthorizedEvents() {
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
func (o *testOrchestrator) initAuthorizedEvents() {
	channel := channels.RequestChunks
	for i := uint64(0); i < numOfAuthorizedEvents; i++ {
		chunkDataReq := &messages.ChunkDataRequest{
			ChunkID: unittest.IdentifierFixture(),
			Nonce:   rand.Uint64(),
		}
		eventID := o.getFlowProtocolEventID(channel, chunkDataReq)
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
		o.authorizedEventsWg.Add(1)
	}
}

// initUnauthorizedMsgByRoleEvents sets n number of events where the sender is unauthorized to
// send the FlowProtocolEvent. In this case AN's are not authorized to send block proposals.
func (o *testOrchestrator) initUnauthorizedMsgByRoleEvents(n int) {
	channel := channels.SyncCommittee
	for i := 0; i < n; i++ {
		unauthorizedProposal := unittest.ProposalFixture()
		eventID := o.getFlowProtocolEventID(channel, unauthorizedProposal)
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
func (o *testOrchestrator) initUnauthorizedMsgOnChannelEvents(n int) {
	channel := channels.PushReceipts
	for i := 0; i < n; i++ {
		syncReq := &messages.SyncRequest{
			Nonce:  rand.Uint64(),
			Height: rand.Uint64(),
		}
		eventID := o.getFlowProtocolEventID(channel, syncReq)
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
func (o *testOrchestrator) initUnauthorizedUnicastOnChannelEvents(n int) {
	channel := channels.SyncCommittee
	for i := 0; i < n; i++ {
		syncReq := &messages.SyncRequest{
			Nonce:  rand.Uint64(),
			Height: rand.Uint64(),
		}
		eventID := o.getFlowProtocolEventID(channel, syncReq)
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
func (o *testOrchestrator) initUnauthorizedPublishOnChannelEvents(n int) {
	channel := channels.ProvideChunks
	for i := 0; i < n; i++ {
		chunkDataResponse := unittest.ChunkDataResponseMsgFixture(unittest.IdentifierFixture())
		eventID := o.getFlowProtocolEventID(channel, chunkDataResponse)
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

func (o *testOrchestrator) getFlowProtocolEventID(channel channels.Channel, event interface{}) string {
	payload, err := o.codec.Encode(event)
	require.NoError(o.t, err)
	eventID, err := p2p.EventId(channel, payload)
	require.NoError(o.t, err)
	return eventID.Hex()
}
