package convert

import (
	"encoding/hex"
	"fmt"

	"github.com/coreos/go-semver/semver"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/assignment"
	"github.com/onflow/flow-go/model/flow/order"
)

// ServiceEvent converts a service event encoded as the generic flow.Event
// type to a flow.ServiceEvent type for use within protocol software and protocol
// state. This acts as the conversion from the Cadence type to the flow-go type.
func ServiceEvent(chainID flow.ChainID, event flow.Event) (*flow.ServiceEvent, error) {

	events := systemcontracts.ServiceEventsForChain(chainID)

	// depending on type of service event construct Go type
	switch event.Type {
	case events.EpochSetup.EventType():
		return convertServiceEventEpochSetup(event)
	case events.EpochCommit.EventType():
		return convertServiceEventEpochCommit(event)
	case events.VersionBeacon.EventType():
		return convertServiceEventVersionBeacon(event)
	default:
		return nil, fmt.Errorf("invalid event type: %s", event.Type)
	}
}

// convertServiceEventEpochSetup converts a service event encoded as the generic
// flow.Event type to a ServiceEvent type for an EpochSetup event
func convertServiceEventEpochSetup(event flow.Event) (*flow.ServiceEvent, error) {

	// decode bytes using ccf
	payload, err := ccf.Decode(nil, event.Payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal event payload: %w", err)
	}

	// NOTE: variable names prefixed with cdc represent cadence types
	cdcEvent, ok := payload.(cadence.Event)
	if !ok {
		return nil, invalidCadenceTypeError("payload", payload, cadence.Event{})
	}

	const expectedFieldCount = 9
	if len(cdcEvent.Fields) < expectedFieldCount {
		return nil, fmt.Errorf(
			"insufficient fields in EpochSetup event (%d < %d)",
			len(cdcEvent.Fields),
			expectedFieldCount,
		)
	}

	if cdcEvent.Type() == nil {
		return nil, fmt.Errorf("EpochSetup event doesn't have type")
	}

	// parse EpochSetup event

	var counter cadence.UInt64
	var firstView cadence.UInt64
	var finalView cadence.UInt64
	var randomSrcHex cadence.String
	var dkgPhase1FinalView cadence.UInt64
	var dkgPhase2FinalView cadence.UInt64
	var dkgPhase3FinalView cadence.UInt64
	var cdcClusters cadence.Array
	var cdcParticipants cadence.Array
	var foundFieldCount int

	evt := cdcEvent.Type().(*cadence.EventType)

	for i, f := range evt.Fields {
		switch f.Identifier {
		case "counter":
			foundFieldCount++
			counter, ok = cdcEvent.Fields[i].(cadence.UInt64)
			if !ok {
				return nil, invalidCadenceTypeError(
					"counter",
					cdcEvent.Fields[i],
					cadence.UInt64(0),
				)
			}

		case "nodeInfo":
			foundFieldCount++
			cdcParticipants, ok = cdcEvent.Fields[i].(cadence.Array)
			if !ok {
				return nil, invalidCadenceTypeError(
					"participants",
					cdcEvent.Fields[i],
					cadence.Array{},
				)
			}

		case "firstView":
			foundFieldCount++
			firstView, ok = cdcEvent.Fields[i].(cadence.UInt64)
			if !ok {
				return nil, invalidCadenceTypeError(
					"firstView",
					cdcEvent.Fields[i],
					cadence.UInt64(0),
				)
			}

		case "finalView":
			foundFieldCount++
			finalView, ok = cdcEvent.Fields[i].(cadence.UInt64)
			if !ok {
				return nil, invalidCadenceTypeError(
					"finalView",
					cdcEvent.Fields[i],
					cadence.UInt64(0),
				)
			}

		case "collectorClusters":
			foundFieldCount++
			cdcClusters, ok = cdcEvent.Fields[i].(cadence.Array)
			if !ok {
				return nil, invalidCadenceTypeError(
					"clusters",
					cdcEvent.Fields[i],
					cadence.Array{},
				)
			}

		case "randomSource":
			foundFieldCount++
			randomSrcHex, ok = cdcEvent.Fields[i].(cadence.String)
			if !ok {
				return nil, invalidCadenceTypeError(
					"randomSource",
					cdcEvent.Fields[i],
					cadence.String(""),
				)
			}

		case "DKGPhase1FinalView":
			foundFieldCount++
			dkgPhase1FinalView, ok = cdcEvent.Fields[i].(cadence.UInt64)
			if !ok {
				return nil, invalidCadenceTypeError(
					"dkgPhase1FinalView",
					cdcEvent.Fields[i],
					cadence.UInt64(0),
				)
			}

		case "DKGPhase2FinalView":
			foundFieldCount++
			dkgPhase2FinalView, ok = cdcEvent.Fields[i].(cadence.UInt64)
			if !ok {
				return nil, invalidCadenceTypeError(
					"dkgPhase2FinalView",
					cdcEvent.Fields[i],
					cadence.UInt64(0),
				)
			}

		case "DKGPhase3FinalView":
			foundFieldCount++
			dkgPhase3FinalView, ok = cdcEvent.Fields[i].(cadence.UInt64)
			if !ok {
				return nil, invalidCadenceTypeError(
					"dkgPhase3FinalView",
					cdcEvent.Fields[i],
					cadence.UInt64(0),
				)
			}
		}
	}

	if foundFieldCount != expectedFieldCount {
		return nil, fmt.Errorf(
			"EpochSetup event required fields not found (%d != %d)",
			foundFieldCount,
			expectedFieldCount,
		)
	}

	setup := &flow.EpochSetup{
		Counter:            uint64(counter),
		FirstView:          uint64(firstView),
		FinalView:          uint64(finalView),
		DKGPhase1FinalView: uint64(dkgPhase1FinalView),
		DKGPhase2FinalView: uint64(dkgPhase2FinalView),
		DKGPhase3FinalView: uint64(dkgPhase3FinalView),
	}

	// Cadence's unsafeRandom().toString() produces a string of variable length.
	// Here we pad it with enough 0s to meet the required length.
	paddedRandomSrcHex := fmt.Sprintf(
		"%0*s",
		2*flow.EpochSetupRandomSourceLength,
		string(randomSrcHex),
	)
	setup.RandomSource, err = hex.DecodeString(paddedRandomSrcHex)
	if err != nil {
		return nil, fmt.Errorf(
			"could not decode random source hex (%v): %w",
			paddedRandomSrcHex,
			err,
		)
	}

	// parse cluster assignments
	setup.Assignments, err = convertClusterAssignments(cdcClusters.Values)
	if err != nil {
		return nil, fmt.Errorf("could not convert cluster assignments: %w", err)
	}

	// parse epoch participants
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

	// decode bytes using ccf
	payload, err := ccf.Decode(nil, event.Payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal event payload: %w", err)
	}

	cdcEvent, ok := payload.(cadence.Event)
	if !ok {
		return nil, invalidCadenceTypeError("payload", payload, cadence.Event{})
	}

	const expectedFieldCount = 3
	if len(cdcEvent.Fields) < expectedFieldCount {
		return nil, fmt.Errorf(
			"insufficient fields in EpochCommit event (%d < %d)",
			len(cdcEvent.Fields),
			expectedFieldCount,
		)
	}

	if cdcEvent.Type() == nil {
		return nil, fmt.Errorf("EpochCommit event doesn't have type")
	}

	// Extract EpochCommit event fields
	var counter cadence.UInt64
	var cdcClusterQCVotes cadence.Array
	var cdcDKGKeys cadence.Array
	var foundFieldCount int

	evt := cdcEvent.Type().(*cadence.EventType)

	for i, f := range evt.Fields {
		switch f.Identifier {
		case "counter":
			foundFieldCount++
			counter, ok = cdcEvent.Fields[i].(cadence.UInt64)
			if !ok {
				return nil, invalidCadenceTypeError(
					"counter",
					cdcEvent.Fields[i],
					cadence.UInt64(0),
				)
			}

		case "clusterQCs":
			foundFieldCount++
			cdcClusterQCVotes, ok = cdcEvent.Fields[i].(cadence.Array)
			if !ok {
				return nil, invalidCadenceTypeError(
					"clusterQCs",
					cdcEvent.Fields[i],
					cadence.Array{},
				)
			}

		case "dkgPubKeys":
			foundFieldCount++
			cdcDKGKeys, ok = cdcEvent.Fields[i].(cadence.Array)
			if !ok {
				return nil, invalidCadenceTypeError(
					"dkgPubKeys",
					cdcEvent.Fields[i],
					cadence.Array{},
				)
			}
		}
	}

	if foundFieldCount != expectedFieldCount {
		return nil, fmt.Errorf(
			"EpochCommit event required fields not found (%d != %d)",
			foundFieldCount,
			expectedFieldCount,
		)
	}

	commit := &flow.EpochCommit{
		Counter: uint64(counter),
	}

	// parse cluster qc votes
	commit.ClusterQCs, err = convertClusterQCVotes(cdcClusterQCVotes.Values)
	if err != nil {
		return nil, fmt.Errorf("could not convert cluster qc votes: %w", err)
	}

	// parse DKG group key and participants
	// Note: this is read in the same order as `DKGClient.SubmitResult` ie. with the group public key first followed by individual keys
	// https://github.com/onflow/flow-go/blob/feature/dkg/module/dkg/client.go#L182-L183
	dkgGroupKey, dkgParticipantKeys, err := convertDKGKeys(cdcDKGKeys.Values)
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
	identifierLists := make([]flow.IdentifierList, len(cdcClusters))
	for _, value := range cdcClusters {

		cdcCluster, ok := value.(cadence.Struct)
		if !ok {
			return nil, invalidCadenceTypeError("cluster", cdcCluster, cadence.Struct{})
		}

		const expectedFieldCount = 2
		if len(cdcCluster.Fields) < expectedFieldCount {
			return nil, fmt.Errorf(
				"insufficient fields (%d < %d)",
				len(cdcCluster.Fields),
				expectedFieldCount,
			)
		}

		if cdcCluster.Type() == nil {
			return nil, fmt.Errorf("cluster struct doesn't have type")
		}

		// Extract cluster fields
		var clusterIndex cadence.UInt16
		var weightsByNodeID cadence.Dictionary
		var foundFieldCount int

		cdcClusterType := cdcCluster.Type().(*cadence.StructType)

		for i, f := range cdcClusterType.Fields {
			switch f.Identifier {
			case "index":
				foundFieldCount++
				clusterIndex, ok = cdcCluster.Fields[i].(cadence.UInt16)
				if !ok {
					return nil, invalidCadenceTypeError(
						"index",
						cdcCluster.Fields[i],
						cadence.UInt16(0),
					)
				}

			case "nodeWeights":
				foundFieldCount++
				weightsByNodeID, ok = cdcCluster.Fields[i].(cadence.Dictionary)
				if !ok {
					return nil, invalidCadenceTypeError(
						"nodeWeights",
						cdcCluster.Fields[i],
						cadence.Dictionary{},
					)
				}
			}
		}

		if foundFieldCount != expectedFieldCount {
			return nil, fmt.Errorf(
				"cluster struct required fields not found (%d != %d)",
				foundFieldCount,
				expectedFieldCount,
			)
		}

		// ensure cluster index is valid
		if int(clusterIndex) >= len(cdcClusters) {
			return nil, fmt.Errorf(
				"invalid cdcCluster index (%d) outside range [0,%d]",
				clusterIndex,
				len(cdcClusters)-1,
			)
		}
		_, dup := indices[uint(clusterIndex)]
		if dup {
			return nil, fmt.Errorf("duplicate cdcCluster index (%d)", clusterIndex)
		}

		// read weights to retrieve node IDs of cdcCluster members
		for _, pair := range weightsByNodeID.Pairs {

			nodeIDString, ok := pair.Key.(cadence.String)
			if !ok {
				return nil, invalidCadenceTypeError(
					"clusterWeights.nodeID",
					pair.Key,
					cadence.String(""),
				)
			}
			nodeID, err := flow.HexStringToIdentifier(string(nodeIDString))
			if err != nil {
				return nil, fmt.Errorf(
					"could not convert hex string to identifer: %w",
					err,
				)
			}

			identifierLists[clusterIndex] = append(identifierLists[clusterIndex], nodeID)
		}
	}

	// sort identifier lists in Canonical order
	assignments := assignment.FromIdentifierLists(identifierLists)

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
			return nil, invalidCadenceTypeError(
				"cdcNodeInfoFields",
				value,
				cadence.Struct{},
			)
		}

		const expectedFieldCount = 14
		if len(cdcNodeInfoStruct.Fields) < expectedFieldCount {
			return nil, fmt.Errorf(
				"insufficient fields (%d < %d)",
				len(cdcNodeInfoStruct.Fields),
				expectedFieldCount,
			)
		}

		if cdcNodeInfoStruct.Type() == nil {
			return nil, fmt.Errorf("nodeInfo struct doesn't have type")
		}

		cdcNodeInfoStructType := cdcNodeInfoStruct.Type().(*cadence.StructType)

		const requiredFieldCount = 6
		var foundFieldCount int

		var nodeIDHex cadence.String
		var role cadence.UInt8
		var address cadence.String
		var networkKeyHex cadence.String
		var stakingKeyHex cadence.String
		var initialWeight cadence.UInt64

		for i, f := range cdcNodeInfoStructType.Fields {
			switch f.Identifier {
			case "id":
				foundFieldCount++
				nodeIDHex, ok = cdcNodeInfoStruct.Fields[i].(cadence.String)
				if !ok {
					return nil, invalidCadenceTypeError(
						"nodeInfo.id",
						cdcNodeInfoStruct.Fields[i],
						cadence.String(""),
					)
				}

			case "role":
				foundFieldCount++
				role, ok = cdcNodeInfoStruct.Fields[i].(cadence.UInt8)
				if !ok {
					return nil, invalidCadenceTypeError(
						"nodeInfo.role",
						cdcNodeInfoStruct.Fields[i],
						cadence.UInt8(0),
					)
				}

			case "networkingAddress":
				foundFieldCount++
				address, ok = cdcNodeInfoStruct.Fields[i].(cadence.String)
				if !ok {
					return nil, invalidCadenceTypeError(
						"nodeInfo.networkingAddress",
						cdcNodeInfoStruct.Fields[i],
						cadence.String(""),
					)
				}

			case "networkingKey":
				foundFieldCount++
				networkKeyHex, ok = cdcNodeInfoStruct.Fields[i].(cadence.String)
				if !ok {
					return nil, invalidCadenceTypeError(
						"nodeInfo.networkingKey",
						cdcNodeInfoStruct.Fields[i],
						cadence.String(""),
					)
				}

			case "stakingKey":
				foundFieldCount++
				stakingKeyHex, ok = cdcNodeInfoStruct.Fields[i].(cadence.String)
				if !ok {
					return nil, invalidCadenceTypeError(
						"nodeInfo.stakingKey",
						cdcNodeInfoStruct.Fields[i],
						cadence.String(""),
					)
				}

			case "initialWeight":
				foundFieldCount++
				initialWeight, ok = cdcNodeInfoStruct.Fields[i].(cadence.UInt64)
				if !ok {
					return nil, invalidCadenceTypeError(
						"nodeInfo.initialWeight",
						cdcNodeInfoStruct.Fields[i],
						cadence.UInt64(0),
					)
				}
			}
		}

		if foundFieldCount != requiredFieldCount {
			return nil, fmt.Errorf(
				"NodeInfo struct required fields not found (%d != %d)",
				foundFieldCount,
				requiredFieldCount,
			)
		}

		if !flow.Role(role).Valid() {
			return nil, fmt.Errorf("invalid role %d", role)
		}

		identity := &flow.Identity{
			Address: string(address),
			Weight:  uint64(initialWeight),
			Role:    flow.Role(role),
		}

		// convert nodeID string into identifier
		identity.NodeID, err = flow.HexStringToIdentifier(string(nodeIDHex))
		if err != nil {
			return nil, fmt.Errorf("could not convert hex string to identifer: %w", err)
		}

		// parse to PublicKey the networking key hex string
		networkKeyBytes, err := hex.DecodeString(string(networkKeyHex))
		if err != nil {
			return nil, fmt.Errorf(
				"could not decode network public key into bytes: %w",
				err,
			)
		}
		identity.NetworkPubKey, err = crypto.DecodePublicKey(
			crypto.ECDSAP256,
			networkKeyBytes,
		)
		if err != nil {
			return nil, fmt.Errorf("could not decode network public key: %w", err)
		}

		// parse to PublicKey the staking key hex string
		stakingKeyBytes, err := hex.DecodeString(string(stakingKeyHex))
		if err != nil {
			return nil, fmt.Errorf(
				"could not decode staking public key into bytes: %w",
				err,
			)
		}
		identity.StakingPubKey, err = crypto.DecodePublicKey(
			crypto.BLSBLS12381,
			stakingKeyBytes,
		)
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
func convertClusterQCVotes(cdcClusterQCs []cadence.Value) (
	[]flow.ClusterQCVoteData,
	error,
) {

	// avoid duplicate indices
	indices := make(map[uint]struct{})
	qcVoteDatas := make([]flow.ClusterQCVoteData, len(cdcClusterQCs))

	// CAUTION: Votes are not validated prior to aggregation. This means a single
	// invalid vote submission will result in a fully invalid QC for that cluster.
	// Votes must be validated by the ClusterQC smart contract.

	for _, cdcClusterQC := range cdcClusterQCs {
		cdcClusterQCStruct, ok := cdcClusterQC.(cadence.Struct)
		if !ok {
			return nil, invalidCadenceTypeError(
				"clusterQC",
				cdcClusterQC,
				cadence.Struct{},
			)
		}

		const expectedFieldCount = 4
		if len(cdcClusterQCStruct.Fields) < expectedFieldCount {
			return nil, fmt.Errorf(
				"insufficient fields (%d < %d)",
				len(cdcClusterQCStruct.Fields),
				expectedFieldCount,
			)
		}

		if cdcClusterQCStruct.Type() == nil {
			return nil, fmt.Errorf("clusterQC struct doesn't have type")
		}

		cdcClusterQCStructType := cdcClusterQCStruct.Type().(*cadence.StructType)

		const requiredFieldCount = 3
		var foundFieldCount int

		var index cadence.UInt16
		var cdcVoterIDs cadence.Array
		var cdcRawVotes cadence.Array

		for i, f := range cdcClusterQCStructType.Fields {
			switch f.Identifier {
			case "index":
				foundFieldCount++
				index, ok = cdcClusterQCStruct.Fields[i].(cadence.UInt16)
				if !ok {
					return nil, invalidCadenceTypeError(
						"ClusterQC.index",
						cdcClusterQCStruct.Fields[i],
						cadence.UInt16(0),
					)
				}

			case "voteSignatures":
				foundFieldCount++
				cdcRawVotes, ok = cdcClusterQCStruct.Fields[i].(cadence.Array)
				if !ok {
					return nil, invalidCadenceTypeError(
						"clusterQC.voteSignatures",
						cdcClusterQCStruct.Fields[i],
						cadence.Array{},
					)
				}

			case "voterIDs":
				foundFieldCount++
				cdcVoterIDs, ok = cdcClusterQCStruct.Fields[i].(cadence.Array)
				if !ok {
					return nil, invalidCadenceTypeError(
						"clusterQC.voterIDs",
						cdcClusterQCStruct.Fields[i],
						cadence.Array{},
					)
				}
			}
		}

		if foundFieldCount != requiredFieldCount {
			return nil, fmt.Errorf(
				"clusterQC struct required fields not found (%d != %d)",
				foundFieldCount,
				requiredFieldCount,
			)
		}

		if int(index) >= len(cdcClusterQCs) {
			return nil, fmt.Errorf(
				"invalid index (%d) not in range [0,%d]",
				index,
				len(cdcClusterQCs),
			)
		}
		_, dup := indices[uint(index)]
		if dup {
			return nil, fmt.Errorf("duplicate cluster QC index (%d)", index)
		}

		voterIDs := make([]flow.Identifier, 0, len(cdcVoterIDs.Values))
		for _, cdcVoterID := range cdcVoterIDs.Values {
			voterIDHex, ok := cdcVoterID.(cadence.String)
			if !ok {
				return nil, invalidCadenceTypeError(
					"clusterQC[i].voterID",
					cdcVoterID,
					cadence.String(""),
				)
			}
			voterID, err := flow.HexStringToIdentifier(string(voterIDHex))
			if err != nil {
				return nil, fmt.Errorf("could not convert voter ID from hex: %w", err)
			}
			voterIDs = append(voterIDs, voterID)
		}

		// gather all the vote signatures
		signatures := make([]crypto.Signature, 0, len(cdcRawVotes.Values))
		for _, cdcRawVote := range cdcRawVotes.Values {
			rawVoteHex, ok := cdcRawVote.(cadence.String)
			if !ok {
				return nil, invalidCadenceTypeError(
					"clusterQC[i].vote",
					cdcRawVote,
					cadence.String(""),
				)
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
			// expected errors of the function are:
			//  - empty list of signatures
			//  - an input signature does not deserialize to a valid point
			// Both are not expected at this stage because list is guaranteed not to be
			// empty and individual signatures have been validated.
			return nil, fmt.Errorf("cluster qc vote aggregation failed: %w", err)
		}

		// check that aggregated signature is not identity, because an identity signature
		// is invalid if verified under an identity public key. This can happen in two cases:
		//  - If the quorum has at least one honest signer, and given all staking key proofs of possession
		//    are valid, it's extremely unlikely for the aggregated public key (and the corresponding
		//    aggregated signature) to be identity.
		//  - If all quorum is malicious and intentionally forge an identity aggregate. As of the previous point,
		//    this is only possible if there is no honest collector involved in constructing the cluster QC.
		//    Hence, the cluster would need to contain a supermajority of malicious collectors.
		//    As we are assuming that the fraction of malicious collectors overall does not exceed 1/3  (measured
		//    by stake), the probability for randomly assigning 2/3 or more byzantine collectors to a single cluster
		//    vanishes (provided a sufficiently high collector count in total).
		//
		//  Note that at this level, all individual signatures are guaranteed to be valid
		//  w.r.t their corresponding staking public key. It is therefore enough to check
		//  the aggregated signature to conclude whether the aggregated public key is identity.
		//  This check is therefore a sanity check to catch a potential issue early.
		if crypto.IsBLSSignatureIdentity(aggregatedSignature) {
			return nil, fmt.Errorf("cluster qc vote aggregation failed because resulting BLS signature is identity")
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
func convertDKGKeys(cdcDKGKeys []cadence.Value) (
	groupKey crypto.PublicKey,
	participantKeys []crypto.PublicKey,
	err error,
) {

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
		return nil, nil, fmt.Errorf(
			"could not decode group public key into bytes: %w",
			err,
		)
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
			return nil, nil, fmt.Errorf(
				"could not decode individual public key into bytes: %w",
				err,
			)
		}
		pubKey, err := crypto.DecodePublicKey(crypto.BLSBLS12381, pubKeyBytes)
		if err != nil {
			return nil, nil, fmt.Errorf("could not decode dkg public key: %w", err)
		}
		dkgParticipantKeys = append(dkgParticipantKeys, pubKey)
	}

	return groupKey, dkgParticipantKeys, nil
}

func invalidCadenceTypeError(
	fieldName string,
	actualType, expectedType cadence.Value,
) error {
	return fmt.Errorf(
		"invalid Cadence type for field %s (got=%s, expected=%s)",
		fieldName,
		actualType.Type().ID(),
		expectedType.Type().ID(),
	)
}

func convertServiceEventVersionBeacon(event flow.Event) (*flow.ServiceEvent, error) {
	payload, err := ccf.Decode(nil, event.Payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal event payload: %w", err)
	}

	versionBeacon, err := DecodeCadenceValue(
		"VersionBeacon payload", payload, func(cdcEvent cadence.Event) (
			flow.VersionBeacon,
			error,
		) {
			const expectedFieldCount = 2
			if len(cdcEvent.Fields) != expectedFieldCount {
				return flow.VersionBeacon{}, fmt.Errorf(
					"unexpected number of fields in VersionBeacon event (%d != %d)",
					len(cdcEvent.Fields),
					expectedFieldCount,
				)
			}

			if cdcEvent.Type() == nil {
				return flow.VersionBeacon{}, fmt.Errorf("VersionBeacon event doesn't have type")
			}

			var versionBoundariesValue, sequenceValue cadence.Value
			var foundFieldCount int

			evt := cdcEvent.Type().(*cadence.EventType)

			for i, f := range evt.Fields {
				switch f.Identifier {
				case "versionBoundaries":
					foundFieldCount++
					versionBoundariesValue = cdcEvent.Fields[i]

				case "sequence":
					foundFieldCount++
					sequenceValue = cdcEvent.Fields[i]
				}
			}

			if foundFieldCount != expectedFieldCount {
				return flow.VersionBeacon{}, fmt.Errorf(
					"VersionBeacon event required fields not found (%d != %d)",
					foundFieldCount,
					expectedFieldCount,
				)
			}

			versionBoundaries, err := DecodeCadenceValue(
				".versionBoundaries", versionBoundariesValue, convertVersionBoundaries,
			)
			if err != nil {
				return flow.VersionBeacon{}, err
			}

			sequence, err := DecodeCadenceValue(
				".sequence", sequenceValue, func(cadenceVal cadence.UInt64) (
					uint64,
					error,
				) {
					return uint64(cadenceVal), nil
				},
			)
			if err != nil {
				return flow.VersionBeacon{}, err
			}

			return flow.VersionBeacon{
				VersionBoundaries: versionBoundaries,
				Sequence:          sequence,
			}, err
		},
	)
	if err != nil {
		return nil, err
	}

	// a converted version beacon event should also be valid
	if err := versionBeacon.Validate(); err != nil {
		return nil, fmt.Errorf("invalid VersionBeacon event: %w", err)
	}

	// create the service event
	serviceEvent := &flow.ServiceEvent{
		Type:  flow.ServiceEventVersionBeacon,
		Event: &versionBeacon,
	}

	return serviceEvent, nil
}

func convertVersionBoundaries(array cadence.Array) (
	[]flow.VersionBoundary,
	error,
) {
	boundaries := make([]flow.VersionBoundary, len(array.Values))

	for i, cadenceVal := range array.Values {
		boundary, err := DecodeCadenceValue(
			fmt.Sprintf(".Values[%d]", i),
			cadenceVal,
			func(structVal cadence.Struct) (
				flow.VersionBoundary,
				error,
			) {
				const expectedFieldCount = 2
				if len(structVal.Fields) < expectedFieldCount {
					return flow.VersionBoundary{}, fmt.Errorf(
						"incorrect number of fields (%d != %d)",
						len(structVal.Fields),
						expectedFieldCount,
					)
				}

				if structVal.Type() == nil {
					return flow.VersionBoundary{}, fmt.Errorf("VersionBoundary struct doesn't have type")
				}

				var blockHeightValue, versionValue cadence.Value
				var foundFieldCount int

				structValType := structVal.Type().(*cadence.StructType)

				for i, f := range structValType.Fields {
					switch f.Identifier {
					case "blockHeight":
						foundFieldCount++
						blockHeightValue = structVal.Fields[i]

					case "version":
						foundFieldCount++
						versionValue = structVal.Fields[i]
					}
				}

				if foundFieldCount != expectedFieldCount {
					return flow.VersionBoundary{}, fmt.Errorf(
						"VersionBoundaries struct required fields not found (%d != %d)",
						foundFieldCount,
						expectedFieldCount,
					)
				}

				height, err := DecodeCadenceValue(
					".blockHeight",
					blockHeightValue,
					func(cadenceVal cadence.UInt64) (
						uint64,
						error,
					) {
						return uint64(cadenceVal), nil
					},
				)
				if err != nil {
					return flow.VersionBoundary{}, err
				}

				version, err := DecodeCadenceValue(
					".version",
					versionValue,
					convertSemverVersion,
				)
				if err != nil {
					return flow.VersionBoundary{}, err
				}

				return flow.VersionBoundary{
					BlockHeight: height,
					Version:     version,
				}, nil
			},
		)
		if err != nil {
			return nil, err
		}
		boundaries[i] = boundary
	}

	return boundaries, nil
}

func convertSemverVersion(structVal cadence.Struct) (
	string,
	error,
) {
	const expectedFieldCount = 4
	if len(structVal.Fields) < expectedFieldCount {
		return "", fmt.Errorf(
			"incorrect number of fields (%d != %d)",
			len(structVal.Fields),
			expectedFieldCount,
		)
	}

	if structVal.Type() == nil {
		return "", fmt.Errorf("Semver struct doesn't have type")
	}

	var majorValue, minorValue, patchValue, preReleaseValue cadence.Value
	var foundFieldCount int

	structValType := structVal.Type().(*cadence.StructType)

	for i, f := range structValType.Fields {
		switch f.Identifier {
		case "major":
			foundFieldCount++
			majorValue = structVal.Fields[i]

		case "minor":
			foundFieldCount++
			minorValue = structVal.Fields[i]

		case "patch":
			foundFieldCount++
			patchValue = structVal.Fields[i]

		case "preRelease":
			foundFieldCount++
			preReleaseValue = structVal.Fields[i]
		}
	}

	if foundFieldCount != expectedFieldCount {
		return "", fmt.Errorf(
			"Semver struct required fields not found (%d != %d)",
			foundFieldCount,
			expectedFieldCount,
		)
	}

	major, err := DecodeCadenceValue(
		".major",
		majorValue,
		func(cadenceVal cadence.UInt8) (
			uint64,
			error,
		) {
			return uint64(cadenceVal), nil
		},
	)
	if err != nil {
		return "", err
	}

	minor, err := DecodeCadenceValue(
		".minor",
		minorValue,
		func(cadenceVal cadence.UInt8) (
			uint64,
			error,
		) {
			return uint64(cadenceVal), nil
		},
	)
	if err != nil {
		return "", err
	}

	patch, err := DecodeCadenceValue(
		".patch",
		patchValue,
		func(cadenceVal cadence.UInt8) (
			uint64,
			error,
		) {
			return uint64(cadenceVal), nil
		},
	)
	if err != nil {
		return "", err
	}

	preRelease, err := DecodeCadenceValue(
		".preRelease",
		preReleaseValue,
		func(cadenceVal cadence.Optional) (
			string,
			error,
		) {
			if cadenceVal.Value == nil {
				return "", nil
			}

			return DecodeCadenceValue(
				"!",
				cadenceVal.Value,
				func(cadenceVal cadence.String) (
					string,
					error,
				) {
					return string(cadenceVal), nil
				},
			)
		},
	)
	if err != nil {
		return "", err
	}

	version := semver.Version{
		Major:      int64(major),
		Minor:      int64(minor),
		Patch:      int64(patch),
		PreRelease: semver.PreRelease(preRelease),
	}

	return version.String(), nil

}

type decodeError struct {
	location string
	err      error
}

func (e decodeError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("decoding error %s: %s", e.location, e.err.Error())
	}
	return fmt.Sprintf("decoding error %s", e.location)
}

func (e decodeError) Unwrap() error {
	return e.err
}

func DecodeCadenceValue[From cadence.Value, Into any](
	location string,
	value cadence.Value,
	decodeInner func(From) (Into, error),
) (Into, error) {
	var defaultInto Into
	if value == nil {
		return defaultInto, decodeError{
			location: location,
			err:      nil,
		}
	}

	convertedValue, is := value.(From)
	if !is {
		return defaultInto, decodeError{
			location: location,
			err: fmt.Errorf(
				"invalid Cadence type (got=%T, expected=%T)",
				value,
				*new(From),
			),
		}
	}

	inner, err := decodeInner(convertedValue)
	if err != nil {
		if err, is := err.(decodeError); is {
			return defaultInto, decodeError{
				location: location + err.location,
				err:      err.err,
			}
		}
		return defaultInto, decodeError{
			location: location,
			err:      err,
		}
	}

	return inner, nil
}
