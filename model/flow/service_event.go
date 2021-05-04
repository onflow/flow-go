package flow

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/crypto"
)

const (
	ServiceEventSetup  = "setup"
	ServiceEventCommit = "commit"
)

// ConvertServiceEvent converts a service event encoded as the generic
// flow.Event type to a ServiceEvent type for use within protocol software
// and protocol state. This acts as the conversion from the Cadence type to
// the flow-go type.
func ConvertServiceEvent(event Event) (*ServiceEvent, error) {

	// depending on type of Epoch event construct Go type
	switch event.Type {
	case EventEpochSetup:
		return ConvertServiceEventEpochSetup(event)
	case EventEpochCommit:
		return ConvertServiceEventEpochCommit(event)
	default:
		return nil, fmt.Errorf("invalid event type: %s", event.Type)
	}
}

// ServiceEvent represents a service event, which is a special event that when
// emitted from a service account smart contract, is propagated to the protocol
// and included in blocks. Service events typically cause changes to the
// protocol state. See EpochSetup and EpochCommit events in this package for examples.
//
// This type represents a generic service event and primarily exists to simplify
// encoding and decoding.
type ServiceEvent struct {
	Type  string
	Event interface{}
}

func (se *ServiceEvent) UnmarshalJSON(b []byte) error {

	var enc map[string]interface{}
	err := json.Unmarshal(b, &enc)
	if err != nil {
		return err
	}

	tp, ok := enc["Type"].(string)
	if !ok {
		return fmt.Errorf("missing type key")
	}
	ev, ok := enc["Event"]
	if !ok {
		return fmt.Errorf("missing event key")
	}

	// re-marshal the event, we'll unmarshal it into the appropriate type
	evb, err := json.Marshal(ev)
	if err != nil {
		return err
	}

	var event interface{}
	switch tp {
	case ServiceEventSetup:
		setup := new(EpochSetup)
		err = json.Unmarshal(evb, setup)
		if err != nil {
			return err
		}
		event = setup
	case ServiceEventCommit:
		commit := new(EpochCommit)
		err = json.Unmarshal(evb, commit)
		if err != nil {
			return err
		}
		event = commit
	default:
		return fmt.Errorf("invalid type: %s", tp)
	}

	*se = ServiceEvent{
		Type:  tp,
		Event: event,
	}
	return nil
}

func (se *ServiceEvent) UnmarshalMsgpack(b []byte) error {

	var enc map[string]interface{}
	err := msgpack.Unmarshal(b, &enc)
	if err != nil {
		return err
	}

	tp, ok := enc["Type"].(string)
	if !ok {
		return fmt.Errorf("missing type key")
	}
	ev, ok := enc["Event"]
	if !ok {
		return fmt.Errorf("missing event key")
	}

	// re-marshal the event, we'll unmarshal it into the appropriate type
	evb, err := msgpack.Marshal(ev)
	if err != nil {
		return err
	}

	var event interface{}
	switch tp {
	case ServiceEventSetup:
		setup := new(EpochSetup)
		err = msgpack.Unmarshal(evb, setup)
		if err != nil {
			return err
		}
		event = setup
	case ServiceEventCommit:
		commit := new(EpochCommit)
		err = msgpack.Unmarshal(evb, commit)
		if err != nil {
			return err
		}
		event = commit
	default:
		return fmt.Errorf("invalid type: %s", tp)
	}

	*se = ServiceEvent{
		Type:  tp,
		Event: event,
	}
	return nil
}

// ConvertServiceEventEpochSetup converts a service event encoded as the generic
// flow.Event type to a ServiceEvent type for an EpochSetup event
func ConvertServiceEventEpochSetup(event Event) (*ServiceEvent, error) {

	// create a service event
	serviceEv := new(ServiceEvent)

	// decode bytes using jsoncdc
	payload, err := jsoncdc.Decode(event.Payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal event payload: %w", err)
	}

	// parse cadence types to required fields
	ev := new(EpochSetup)
	ev.Counter = uint64(payload.(cadence.Event).Fields[0].(cadence.UInt64))
	ev.FirstView = uint64(payload.(cadence.Event).Fields[2].(cadence.UInt64))
	ev.FinalView = uint64(payload.(cadence.Event).Fields[3].(cadence.UInt64))
	ev.RandomSource = []byte(payload.(cadence.Event).Fields[5].(cadence.String))

	// parse cluster assignments to Go types
	collectorClusters := payload.(cadence.Event).Fields[4].(cadence.Array).Values
	assignments := make(AssignmentList, len(collectorClusters))
	for _, value := range collectorClusters {

		cluster := value.(cadence.Struct).Fields

		// read cluster index and weights by node ID to extract NodeID
		clusterIndex := uint(cluster[0].(cadence.UInt16))
		weightsByNodeID := cluster[1].(cadence.Dictionary).Pairs

		for _, pair := range weightsByNodeID {

			nodeIDString := string(pair.Key.(cadence.String))
			nodeID, err := HexStringToIdentifier(nodeIDString)
			if err != nil {
				return nil, fmt.Errorf("could not convert hex string to identifer: %w", err)
			}
			assignments[clusterIndex] = append(assignments[clusterIndex], nodeID)
		}
	}
	ev.Assignments = assignments

	// parse epoch participants
	epochParticipants := payload.(cadence.Event).Fields[1].(cadence.Array).Values
	participants := make(IdentityList, 0, len(epochParticipants))
	for _, value := range epochParticipants {

		nodeInfo := value.(cadence.Struct).Fields

		// create and assign fields to identity from cadence Struct
		identity := new(Identity)
		identity.Role = Role(uint8(nodeInfo[1].(cadence.UInt8)))
		identity.Address = string(nodeInfo[2].(cadence.String))
		identity.Stake = uint64(nodeInfo[5].(cadence.UFix64))

		// convert nodeID string into identifier
		identity.NodeID, err = HexStringToIdentifier(string(nodeInfo[0].(cadence.String)))
		if err != nil {
			return nil, fmt.Errorf("could not convert hex string to identifer: %w", err)
		}

		// parse to PublicKey the networking key hex string
		nkBytes, err := hex.DecodeString(string(nodeInfo[3].(cadence.String)))
		if err != nil {
			return nil, fmt.Errorf("could not decode network public key into bytes: %w", err)
		}
		identity.NetworkPubKey, err = crypto.DecodePublicKey(crypto.ECDSAP256, nkBytes)
		if err != nil {
			return nil, fmt.Errorf("could not decode network public key: %w", err)
		}

		// parse to PublicKey the staking key hex string
		skBytes, err := hex.DecodeString(string(nodeInfo[4].(cadence.String)))
		if err != nil {
			return nil, fmt.Errorf("could not decode staking public key into bytes: %w", err)
		}
		identity.StakingPubKey, err = crypto.DecodePublicKey(crypto.BLSBLS12381, skBytes)
		if err != nil {
			return nil, fmt.Errorf("could not decode staking public key: %w", err)
		}
	}
	ev.Participants = participants

	serviceEv.Type = ServiceEventSetup
	serviceEv.Event = ev

	return serviceEv, nil
}

// ConvertServiceEventEpochCommit converts a service event encoded as the generic
// flow.Event type to a ServiceEvent type for an EpochCommit event
func ConvertServiceEventEpochCommit(event Event) (*ServiceEvent, error) {

	// create a service event
	serviceEv := new(ServiceEvent)

	// decode bytes using jsoncdc
	payload, err := jsoncdc.Decode(event.Payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal event payload: %w", err)
	}

	// parse candece types to Go types
	ev := new(EpochCommit)
	ev.Counter = uint64(payload.(cadence.Event).Fields[0].(cadence.UInt64))

	// TODO: parse cluster QC from event

	// parse DKG group key and participants
	// Note: this is read in the same order as `DKGClient.SubmitResult` ie. with the group public key first followed by individual keys
	// https://github.com/onflow/flow-go/blob/feature/dkg/module/dkg/client.go#L182-L183
	dkgValues := payload.(cadence.Event).Fields[2].(cadence.Array).Values
	dkgKeys := make([]string, 0, len(dkgValues))
	for _, value := range dkgValues {
		key := string(value.(cadence.String))
		dkgKeys = append(dkgKeys, key)
	}

	// pop first element - group public key hex string
	groupPubKeyString, dkgKeys := dkgKeys[0], dkgKeys[1:]

	// decode group public key
	groupKeyBytes, err := hex.DecodeString(groupPubKeyString)
	if err != nil {
		return nil, fmt.Errorf("could not decode group public key into bytes: %w", err)
	}
	ev.DKGGroupKey, err = crypto.DecodePublicKey(crypto.BLSBLS12381, groupKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("could not decode group public key: %w", err)
	}

	// decode individual public keys
	dkgParticipantKeys := make([]crypto.PublicKey, 0, len(dkgKeys))
	for _, pubKeyString := range dkgKeys {

		pubKeyBytes, err := hex.DecodeString(pubKeyString)
		if err != nil {
			return nil, fmt.Errorf("could not decode individual public key into bytes: %w", err)
		}
		pubKey, err := crypto.DecodePublicKey(crypto.BLSBLS12381, pubKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("could not decode dkg public key: %w", err)
		}
		dkgParticipantKeys = append(dkgParticipantKeys, pubKey)
	}

	// ev.DKGParticipantKeys = dkgParticipantKeys

	serviceEv.Type = ServiceEventCommit
	serviceEv.Event = ev

	return serviceEv, nil
}
