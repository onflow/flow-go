package convert

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
)

// ServiceEvent converts a service event encoded as the generic flow.Event
// type to a flow.ServiceEvent type for use within protocol software and protocol
// state. This acts as the conversion from the Cadence type to the flow-go type.
func ServiceEvent(chainID flow.ChainID, event flow.Event) (*flow.ServiceEvent, error) {

	events, err := systemcontracts.ServiceEventsForChain(chainID)
	if err != nil {
		return nil, fmt.Errorf("could not get service event info: %w", err)
	}

	// depending on type of service event construct Go type
	switch event.Type {
	case events.EpochSetup.EventType():
		return convertServiceEventEpochSetup(event)
	case events.EpochCommit.EventType():
		return convertServiceEventEpochCommit(event)
	default:
		return nil, fmt.Errorf("invalid event type: %s", event.Type)
	}
}

// convertServiceEventEpochSetup converts a service event encoded as the generic
// flow.Event type to a ServiceEvent type for an EpochSetup event
func convertServiceEventEpochSetup(event flow.Event) (*flow.ServiceEvent, error) {

	// decode bytes using jsoncdc
	payload, err := json.Decode(event.Payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal event payload: %w", err)
	}

	// parse cadence types to required fields
	setup := new(flow.EpochSetup)

	// NOTE: variable names prefixed with cdc represent cadence types
	cdcEvent, ok := payload.(cadence.Event)
	if !ok {
		return nil, invalidCadenceTypeError("payload", payload, cadence.Event{})
	}

	if len(cdcEvent.Fields) < 9 {
		return nil, fmt.Errorf("insufficient fields in EpochSetup event (%d < 9)", len(cdcEvent.Fields))
	}

	// extract simple fields
	counter, ok := cdcEvent.Fields[0].(cadence.UInt64)
	if !ok {
		return nil, invalidCadenceTypeError("counter", cdcEvent.Fields[0], cadence.UInt64(0))
	}
	setup.Counter = uint64(counter)
	firstView, ok := cdcEvent.Fields[2].(cadence.UInt64)
	if !ok {
		return nil, invalidCadenceTypeError("firstView", cdcEvent.Fields[2], cadence.UInt64(0))
	}
	setup.FirstView = uint64(firstView)
	finalView, ok := cdcEvent.Fields[3].(cadence.UInt64)
	if !ok {
		return nil, invalidCadenceTypeError("finalView", cdcEvent.Fields[3], cadence.UInt64(0))
	}
	setup.FinalView = uint64(finalView)
	randomSrcHex, ok := cdcEvent.Fields[5].(cadence.String)
	if !ok {
		return nil, invalidCadenceTypeError("randomSource", cdcEvent.Fields[5], cadence.String(""))
	}
	// Cadence's unsafeRandom().toString() produces a string of variable length.
	// Here we pad it with enough 0s to meet the required length.
	paddedRandomSrcHex := fmt.Sprintf("%0*s", 2*flow.EpochSetupRandomSourceLength, string(randomSrcHex))
	setup.RandomSource, err = hex.DecodeString(paddedRandomSrcHex)
	if err != nil {
		return nil, fmt.Errorf("could not decode random source hex (%v): %w", paddedRandomSrcHex, err)
	}

	dkgPhase1FinalView, ok := cdcEvent.Fields[6].(cadence.UInt64)
	if !ok {
		return nil, invalidCadenceTypeError("dkgPhase1FinalView", cdcEvent.Fields[6], cadence.UInt64(0))
	}
	setup.DKGPhase1FinalView = uint64(dkgPhase1FinalView)
	dkgPhase2FinalView, ok := cdcEvent.Fields[7].(cadence.UInt64)
	if !ok {
		return nil, invalidCadenceTypeError("dkgPhase2FinalView", cdcEvent.Fields[7], cadence.UInt64(0))
	}
	setup.DKGPhase2FinalView = uint64(dkgPhase2FinalView)
	dkgPhase3FinalView, ok := cdcEvent.Fields[8].(cadence.UInt64)
	if !ok {
		return nil, invalidCadenceTypeError("dkgPhase3FinalView", cdcEvent.Fields[8], cadence.UInt64(0))
	}
	setup.DKGPhase3FinalView = uint64(dkgPhase3FinalView)

	// parse cluster assignments
	cdcClusters, ok := cdcEvent.Fields[4].(cadence.Array)
	if !ok {
		return nil, invalidCadenceTypeError("clusters", cdcEvent.Fields[4], cadence.Array{})
	}
	setup.Assignments, err = convertClusterAssignments(cdcClusters.Values)
	if err != nil {
		return nil, fmt.Errorf("could not convert cluster assignments: %w", err)
	}

	// parse epoch participants
	cdcParticipants, ok := cdcEvent.Fields[1].(cadence.Array)
	if !ok {
		return nil, invalidCadenceTypeError("participants", cdcEvent.Fields[1], cadence.Array{})
	}
	setup.Participants, err = convertParticipants(cdcParticipants.Values)
	if err != nil {
		return nil, fmt.Errorf("could not convert participants: %w", err)
	}

	// construct the service event
	serviceEvent := &flow.ServiceEvent{
		Type:  flow.ServiceEventSetup,
		Event: setup,
	}

	return serviceEvent, nil
}

// convertServiceEventEpochCommit converts a service event encoded as the generic
// flow.Event type to a ServiceEvent type for an EpochCommit event
func convertServiceEventEpochCommit(event flow.Event) (*flow.ServiceEvent, error) {

	// decode bytes using jsoncdc
	payload, err := json.Decode(event.Payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal event payload: %w", err)
	}

	// parse cadence types to Go types
	commit := new(flow.EpochCommit)
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
	serviceEvent := &flow.ServiceEvent{
		Type:  flow.ServiceEventCommit,
		Event: commit,
	}

	return serviceEvent, nil
}

// convertClusterAssignments converts the Cadence representation of cluster
// assignments included in the EpochSetup into the protocol AssignmentList
// representation.
func convertClusterAssignments(cdcClusters []cadence.Value) (flow.AssignmentList, error) {

	// ensure we don't have duplicate cluster indices
	indices := make(map[uint]struct{})

	// parse cluster assignments to Go types
	assignments := make(flow.AssignmentList, len(cdcClusters))
	for _, value := range cdcClusters {

		cdcCluster, ok := value.(cadence.Struct)
		if !ok {
			return nil, invalidCadenceTypeError("cluster", cdcCluster, cadence.Struct{})
		}

		expectedFields := 2
		if len(cdcCluster.Fields) < expectedFields {
			return nil, fmt.Errorf("insufficient fields (%d < %d)", len(cdcCluster.Fields), expectedFields)
		}

		// ensure cluster index is valid
		clusterIndex, ok := cdcCluster.Fields[0].(cadence.UInt16)
		if !ok {
			return nil, invalidCadenceTypeError("clusterIndex", cdcCluster.Fields[0], cadence.UInt16(0))
		}
		if int(clusterIndex) >= len(cdcClusters) {
			return nil, fmt.Errorf("invalid cdcCluster index (%d) outside range [0,%d]", clusterIndex, len(cdcClusters)-1)
		}
		_, dup := indices[uint(clusterIndex)]
		if dup {
			return nil, fmt.Errorf("duplicate cdcCluster index (%d)", clusterIndex)
		}

		// read weights to retrieve node IDs of cdcCluster members
		weightsByNodeID, ok := cdcCluster.Fields[1].(cadence.Dictionary)
		if !ok {
			return nil, invalidCadenceTypeError("clusterWeights", cdcCluster.Fields[1], cadence.Dictionary{})
		}

		for _, pair := range weightsByNodeID.Pairs {

			nodeIDString, ok := pair.Key.(cadence.String)
			if !ok {
				return nil, invalidCadenceTypeError("clusterWeights.nodeID", pair.Key, cadence.String(""))
			}
			nodeID, err := flow.HexStringToIdentifier(string(nodeIDString))
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
func convertParticipants(cdcParticipants []cadence.Value) (flow.IdentityList, error) {

	participants := make(flow.IdentityList, 0, len(cdcParticipants))
	var err error

	for _, value := range cdcParticipants {

		cdcNodeInfoStruct, ok := value.(cadence.Struct)
		if !ok {
			return nil, invalidCadenceTypeError("cdcNodeInfoFields", value, cadence.Struct{})
		}
		cdcNodeInfoFields := cdcNodeInfoStruct.Fields

		expectedFields := 14
		if len(cdcNodeInfoFields) < expectedFields {
			return nil, fmt.Errorf("insufficient fields (%d < %d)", len(cdcNodeInfoFields), expectedFields)
		}

		// create and assign fields to identity from cadence Struct
		identity := new(flow.Identity)
		role, ok := cdcNodeInfoFields[1].(cadence.UInt8)
		if !ok {
			return nil, invalidCadenceTypeError("nodeInfo.role", cdcNodeInfoFields[1], cadence.UInt8(0))
		}
		identity.Role = flow.Role(role)
		if !identity.Role.Valid() {
			return nil, fmt.Errorf("invalid role %d", role)
		}

		address, ok := cdcNodeInfoFields[2].(cadence.String)
		if !ok {
			return nil, invalidCadenceTypeError("nodeInfo.address", cdcNodeInfoFields[2], cadence.String(""))
		}
		identity.Address = string(address)

		initialWeight, ok := cdcNodeInfoFields[13].(cadence.UInt64)
		if !ok {
			return nil, invalidCadenceTypeError("nodeInfo.initialWeight", cdcNodeInfoFields[13], cadence.UInt64(0))
		}
		identity.Weight = uint64(initialWeight)

		// convert nodeID string into identifier
		nodeIDHex, ok := cdcNodeInfoFields[0].(cadence.String)
		if !ok {
			return nil, invalidCadenceTypeError("nodeInfo.id", cdcNodeInfoFields[0], cadence.String(""))
		}
		identity.NodeID, err = flow.HexStringToIdentifier(string(nodeIDHex))
		if err != nil {
			return nil, fmt.Errorf("could not convert hex string to identifer: %w", err)
		}

		// parse to PublicKey the networking key hex string
		networkKeyHex, ok := cdcNodeInfoFields[3].(cadence.String)
		if !ok {
			return nil, invalidCadenceTypeError("nodeInfo.networkKey", cdcNodeInfoFields[3], cadence.String(""))
		}
		networkKeyBytes, err := hex.DecodeString(string(networkKeyHex))
		if err != nil {
			return nil, fmt.Errorf("could not decode network public key into bytes: %w", err)
		}
		identity.NetworkPubKey, err = crypto.DecodePublicKey(crypto.ECDSAP256, networkKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("could not decode network public key: %w", err)
		}

		// parse to PublicKey the staking key hex string
		stakingKeyHex, ok := cdcNodeInfoFields[4].(cadence.String)
		if !ok {
			return nil, invalidCadenceTypeError("nodeInfo.stakingKey", cdcNodeInfoFields[4], cadence.String(""))
		}
		stakingKeyBytes, err := hex.DecodeString(string(stakingKeyHex))
		if err != nil {
			return nil, fmt.Errorf("could not decode staking public key into bytes: %w", err)
		}
		identity.StakingPubKey, err = crypto.DecodePublicKey(crypto.BLSBLS12381, stakingKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("could not decode staking public key: %w", err)
		}

		participants = append(participants, identity)
	}

	participants = participants.Sort(order.Canonical)
	return participants, nil
}

// convertClusterQCVotes converts raw cluster QC votes from the EpochCommit event
// to a representation suitable for inclusion in the protocol state. Votes are
// aggregated as part of this conversion.
func convertClusterQCVotes(cdcClusterQCs []cadence.Value) ([]flow.ClusterQCVoteData, error) {

	// avoid duplicate indices
	indices := make(map[uint]struct{})
	qcVoteDatas := make([]flow.ClusterQCVoteData, len(cdcClusterQCs))

	// CAUTION: Votes are not validated prior to aggregation. This means a single
	// invalid vote submission will result in a fully invalid QC for that cluster.
	// Votes must be validated by the ClusterQC smart contract.

	for _, cdcClusterQC := range cdcClusterQCs {
		cdcClusterQCStruct, ok := cdcClusterQC.(cadence.Struct)
		if !ok {
			return nil, invalidCadenceTypeError("clusterQC", cdcClusterQC, cadence.Struct{})
		}
		cdcClusterQCFields := cdcClusterQCStruct.Fields

		expectedFields := 4
		if len(cdcClusterQCFields) < expectedFields {
			return nil, fmt.Errorf("insufficient fields (%d < %d)", len(cdcClusterQCFields), expectedFields)
		}

		index, ok := cdcClusterQCFields[0].(cadence.UInt16)
		if !ok {
			return nil, invalidCadenceTypeError("clusterQC.index", cdcClusterQCFields[0], cadence.UInt16(0))
		}
		if int(index) >= len(cdcClusterQCs) {
			return nil, fmt.Errorf("invalid index (%d) not in range [0,%d]", index, len(cdcClusterQCs))
		}
		_, dup := indices[uint(index)]
		if dup {
			return nil, fmt.Errorf("duplicate cluster QC index (%d)", index)
		}

		cdcVoterIDs, ok := cdcClusterQCFields[3].(cadence.Array)
		if !ok {
			return nil, invalidCadenceTypeError("clusterQC.voterIDs", cdcClusterQCFields[2], cadence.Array{})
		}

		voterIDs := make([]flow.Identifier, 0, len(cdcVoterIDs.Values))
		for _, cdcVoterID := range cdcVoterIDs.Values {
			voterIDHex, ok := cdcVoterID.(cadence.String)
			if !ok {
				return nil, invalidCadenceTypeError("clusterQC[i].voterID", cdcVoterID, cadence.String(""))
			}
			voterID, err := flow.HexStringToIdentifier(string(voterIDHex))
			if err != nil {
				return nil, fmt.Errorf("could not convert voter ID from hex: %w", err)
			}
			voterIDs = append(voterIDs, voterID)
		}

		// gather all the vote signatures
		cdcRawVotes := cdcClusterQCFields[1].(cadence.Array)
		signatures := make([]crypto.Signature, 0, len(cdcRawVotes.Values))
		for _, cdcRawVote := range cdcRawVotes.Values {
			rawVoteHex, ok := cdcRawVote.(cadence.String)
			if !ok {
				return nil, invalidCadenceTypeError("clusterQC[i].vote", cdcRawVote, cadence.String(""))
			}
			rawVoteBytes, err := hex.DecodeString(string(rawVoteHex))
			if err != nil {
				return nil, fmt.Errorf("could not convert raw vote from hex: %w", err)
			}
			signatures = append(signatures, rawVoteBytes)
		}
		// Aggregate BLS signatures
		aggregatedSignature, err := crypto.AggregateBLSSignatures(signatures)
		if err != nil {
			return nil, fmt.Errorf("cluster qc vote aggregation failed: %w", err)
		}

		// set the fields on the QC vote data object
		qcVoteDatas[int(index)] = flow.ClusterQCVoteData{
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
		keyHex, ok := value.(cadence.String)
		if !ok {
			return nil, nil, invalidCadenceTypeError("dkgKey", value, cadence.String(""))
		}
		hexDKGKeys = append(hexDKGKeys, string(keyHex))
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

func invalidCadenceTypeError(fieldName string, actualType, expectedType cadence.Value) error {
	return fmt.Errorf("invalid Cadence type for field %s (got=%s, expected=%s)",
		fieldName,
		actualType.Type().ID(),
		expectedType.Type().ID())
}
