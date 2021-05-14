package flow

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/onflow/flow-go/module/signature"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow/order"
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

	// decode bytes using jsoncdc
	payload, err := jsoncdc.Decode(event.Payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal event payload: %w", err)
	}

	// parse cadence types to required fields
	setup := new(EpochSetup)

	// NOTE: variable names prefixed with cdc represent cadence types
	cdcEvent := payload.(cadence.Event)

	// extract simple fields
	setup.Counter = uint64(cdcEvent.Fields[0].(cadence.UInt64))
	setup.FirstView = uint64(cdcEvent.Fields[2].(cadence.UInt64))
	setup.FinalView = uint64(cdcEvent.Fields[3].(cadence.UInt64))
	randomSrcHex := string(cdcEvent.Fields[5].(cadence.String))
	setup.RandomSource, err = hex.DecodeString(randomSrcHex)
	if err != nil {
		return nil, fmt.Errorf("could not decode random source hex: %w", err)
	}

	// parse cluster assignments
	cdcClusters := cdcEvent.Fields[4].(cadence.Array).Values
	assignments, err := convertClusterAssignments(cdcClusters)
	if err != nil {
		return nil, fmt.Errorf("could not convert cluster assignments: %w", err)
	}
	setup.Assignments = assignments

	// parse epoch participants
	cdcParticipants := cdcEvent.Fields[1].(cadence.Array).Values
	setup.Participants, err = convertParticipants(cdcParticipants)
	if err != nil {
		return nil, fmt.Errorf("could not convert participants: %w", err)
	}

	// construct the service event
	serviceEvent := &ServiceEvent{
		Type:  ServiceEventSetup,
		Event: setup,
	}

	return serviceEvent, nil
}

// convertClusterAssignments converts the Cadence representation of cluster
// assignments included in the EpochSetup into the protocol AssignmentList
// representation.
func convertClusterAssignments(cdcClusters []cadence.Value) (AssignmentList, error) {

	// ensure we don't have duplicate cluster indices
	indices := make(map[uint]struct{})

	// parse cluster assignments to Go types
	assignments := make(AssignmentList, len(cdcClusters))
	for _, value := range cdcClusters {

		cluster := value.(cadence.Struct).Fields

		// ensure cluster index is valid
		clusterIndex := uint(cluster[0].(cadence.UInt16))
		if int(clusterIndex) >= len(cdcClusters) {
			return nil, fmt.Errorf("invalid cluster index (%d) outside range [0,%d]", clusterIndex, len(cdcClusters)-1)
		}
		_, dup := indices[clusterIndex]
		if dup {
			return nil, fmt.Errorf("duplicate cluster index (%d)", clusterIndex)
		}

		// read weights to retrieve node IDs of cluster members
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

	return assignments, nil
}

// convertParticipants converts the network participants specified in the
// EpochSetup event into an IdentityList.
func convertParticipants(cdcParticipants []cadence.Value) (IdentityList, error) {

	participants := make(IdentityList, 0, len(cdcParticipants))
	var err error

	for _, value := range cdcParticipants {

		nodeInfo := value.(cadence.Struct).Fields

		// create and assign fields to identity from cadence Struct
		identity := new(Identity)
		identity.Role = Role(nodeInfo[1].(cadence.UInt8))
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

		participants = append(participants, identity)
	}

	participants = participants.Sort(order.Canonical)
	return participants, nil
}

// ConvertServiceEventEpochCommit converts a service event encoded as the generic
// flow.Event type to a ServiceEvent type for an EpochCommit event
func ConvertServiceEventEpochCommit(event Event) (*ServiceEvent, error) {

	// decode bytes using jsoncdc
	payload, err := jsoncdc.Decode(event.Payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal event payload: %w", err)
	}

	// parse cadence types to Go types
	commit := new(EpochCommit)
	commit.Counter = uint64(payload.(cadence.Event).Fields[0].(cadence.UInt64))

	// parse cluster qc votes
	cdcClusterQCVotes := payload.(cadence.Event).Fields[1].(cadence.Array).Values
	commit.ClusterQCs, err = convertClusterQCVotes(cdcClusterQCVotes)
	if err != nil {
		return nil, fmt.Errorf("could not convert cluster qc votes: %w", err)
	}

	// parse DKG group key and participants
	// Note: this is read in the same order as `DKGClient.SubmitResult` ie. with the group public key first followed by individual keys
	// https://github.com/onflow/flow-go/blob/feature/dkg/module/dkg/client.go#L182-L183
	cdcDKGKeys := payload.(cadence.Event).Fields[2].(cadence.Array).Values
	dkgGroupKey, dkgParticipantKeys, err := convertDKGKeys(cdcDKGKeys)
	if err != nil {
		return nil, fmt.Errorf("could not convert DKG keys: %w", err)
	}

	commit.DKGGroupKey = dkgGroupKey
	commit.DKGParticipantKeys = dkgParticipantKeys

	// create the service event
	serviceEvent := &ServiceEvent{
		Type:  ServiceEventCommit,
		Event: commit,
	}

	return serviceEvent, nil
}

// convertClusterQCVotes converts raw cluster QC votes from the EpochCommit event
// to a representation suitable for inclusion in the protocol state. Votes are
// aggregated as part of this conversion.
func convertClusterQCVotes(cdcClusterQCs []cadence.Value) ([]ClusterQCVoteData, error) {

	// avoid duplicate indices
	indices := make(map[uint]struct{})
	qcVoteDatas := make([]ClusterQCVoteData, len(cdcClusterQCs))

	// CAUTION: Votes are not validated prior to aggregation. This means a single
	// invalid vote submission will result in a fully invalid QC for that cluster.
	// Votes should be validated upon submission by the ClusterQC smart contract.
	// TODO issue for the above
	//
	// NOTE: Aggregation doesn't require a tag or local, but is only accessible
	// through the broader Provider API, hence the empty arguments.
	aggregator := signature.NewAggregationProvider("", nil)

	for _, cdcClusterQC := range cdcClusterQCs {
		index := uint(cdcClusterQC.(cadence.Struct).Fields[0].(cadence.UInt16))
		if int(index) >= len(cdcClusterQCs) {
			return nil, fmt.Errorf("invalid index (%d) not in range [0,%d]", index, len(cdcClusterQCs))
		}
		_, dup := indices[index]
		if dup {
			return nil, fmt.Errorf("duplicate cluster QC index (%d)", index)
		}

		cdcVoterIDs := cdcClusterQC.(cadence.Struct).Fields[2].(cadence.Array).Values
		voterIDs := make([]Identifier, 0, len(cdcVoterIDs))
		for _, cdcVoterID := range cdcVoterIDs {
			voterIDHex := string(cdcVoterID.(cadence.String))
			voterID, err := HexStringToIdentifier(voterIDHex)
			if err != nil {
				return nil, fmt.Errorf("could not convert voter ID from hex: %w", err)
			}
			voterIDs = append(voterIDs, voterID)
		}

		// gather all the vote signatures
		cdcRawVotes := cdcClusterQC.(cadence.Struct).Fields[1].(cadence.Array).Values
		signatures := make([]crypto.Signature, 0, len(cdcRawVotes))
		for _, cdcRawVote := range cdcRawVotes {
			rawVoteHex := string(cdcRawVote.(cadence.String))
			rawVoteBytes, err := hex.DecodeString(rawVoteHex)
			if err != nil {
				return nil, fmt.Errorf("could not convert raw vote from hex: %w", err)
			}
			signatures = append(signatures, rawVoteBytes)
		}
		aggregatedSignature, err := aggregator.Aggregate(signatures)
		if err != nil {
			return nil, fmt.Errorf("cluster qc vote aggregation failed: %w", err)
		}

		// set the fields on the QC vote data object
		qcVoteDatas[int(index)] = ClusterQCVoteData{
			SigData:  aggregatedSignature,
			VoterIDs: voterIDs,
		}
	}

	return qcVoteDatas, nil
}

// convertDKGKeys converts hex-encoded DKG public keys as received by the DKG
// smart contract into crypto.PublicKey representations suitable for inclusion
// in the protocol state.
func convertDKGKeys(cdcDKGKeys []cadence.Value) (groupKey crypto.PublicKey, participantKeys []crypto.PublicKey, err error) {

	hexDKGKeys := make([]string, 0, len(cdcDKGKeys))
	for _, value := range cdcDKGKeys {
		key := string(value.(cadence.String))
		hexDKGKeys = append(hexDKGKeys, key)
	}

	// pop first element - group public key hex string
	groupPubKeyHex := hexDKGKeys[0]
	hexDKGKeys = hexDKGKeys[1:]

	// decode group public key
	groupKeyBytes, err := hex.DecodeString(groupPubKeyHex)
	if err != nil {
		return nil, nil, fmt.Errorf("could not decode group public key into bytes: %w", err)
	}
	groupKey, err = crypto.DecodePublicKey(crypto.BLSBLS12381, groupKeyBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("could not decode group public key: %w", err)
	}

	// decode individual public keys
	dkgParticipantKeys := make([]crypto.PublicKey, 0, len(hexDKGKeys))
	for _, pubKeyString := range hexDKGKeys {

		pubKeyBytes, err := hex.DecodeString(pubKeyString)
		if err != nil {
			return nil, nil, fmt.Errorf("could not decode individual public key into bytes: %w", err)
		}
		pubKey, err := crypto.DecodePublicKey(crypto.BLSBLS12381, pubKeyBytes)
		if err != nil {
			return nil, nil, fmt.Errorf("could not decode dkg public key: %w", err)
		}
		dkgParticipantKeys = append(dkgParticipantKeys, pubKey)
	}

	return groupKey, dkgParticipantKeys, nil
}
