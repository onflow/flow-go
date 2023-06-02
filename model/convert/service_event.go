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

	// parse cadence types to required fields
	setup := new(flow.EpochSetup)

	// NOTE: variable names prefixed with cdc represent cadence types
	cdcEvent, ok := payload.(cadence.Event)
	if !ok {
		return nil, invalidCadenceTypeError("payload", payload, cadence.Event{})
	}

	if len(cdcEvent.Fields) < 9 {
		return nil, fmt.Errorf(
			"insufficient fields in EpochSetup event (%d < 9)",
			len(cdcEvent.Fields),
		)
	}

	// extract simple fields
	counter, ok := cdcEvent.Fields[0].(cadence.UInt64)
	if !ok {
		return nil, invalidCadenceTypeError(
			"counter",
			cdcEvent.Fields[0],
			cadence.UInt64(0),
		)
	}
	setup.Counter = uint64(counter)
	firstView, ok := cdcEvent.Fields[2].(cadence.UInt64)
	if !ok {
		return nil, invalidCadenceTypeError(
			"firstView",
			cdcEvent.Fields[2],
			cadence.UInt64(0),
		)
	}
	setup.FirstView = uint64(firstView)
	finalView, ok := cdcEvent.Fields[3].(cadence.UInt64)
	if !ok {
		return nil, invalidCadenceTypeError(
			"finalView",
			cdcEvent.Fields[3],
			cadence.UInt64(0),
		)
	}
	setup.FinalView = uint64(finalView)
	randomSrcHex, ok := cdcEvent.Fields[5].(cadence.String)
	if !ok {
		return nil, invalidCadenceTypeError(
			"randomSource",
			cdcEvent.Fields[5],
			cadence.String(""),
		)
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

	dkgPhase1FinalView, ok := cdcEvent.Fields[6].(cadence.UInt64)
	if !ok {
		return nil, invalidCadenceTypeError(
			"dkgPhase1FinalView",
			cdcEvent.Fields[6],
			cadence.UInt64(0),
		)
	}
	setup.DKGPhase1FinalView = uint64(dkgPhase1FinalView)
	dkgPhase2FinalView, ok := cdcEvent.Fields[7].(cadence.UInt64)
	if !ok {
		return nil, invalidCadenceTypeError(
			"dkgPhase2FinalView",
			cdcEvent.Fields[7],
			cadence.UInt64(0),
		)
	}
	setup.DKGPhase2FinalView = uint64(dkgPhase2FinalView)
	dkgPhase3FinalView, ok := cdcEvent.Fields[8].(cadence.UInt64)
	if !ok {
		return nil, invalidCadenceTypeError(
			"dkgPhase3FinalView",
			cdcEvent.Fields[8],
			cadence.UInt64(0),
		)
	}
	setup.DKGPhase3FinalView = uint64(dkgPhase3FinalView)

	// parse cluster assignments
	cdcClusters, ok := cdcEvent.Fields[4].(cadence.Array)
	if !ok {
		return nil, invalidCadenceTypeError(
			"clusters",
			cdcEvent.Fields[4],
			cadence.Array{},
		)
	}
	setup.Assignments, err = convertClusterAssignments(cdcClusters.Values)
	if err != nil {
		return nil, fmt.Errorf("could not convert cluster assignments: %w", err)
	}

	// parse epoch participants
	cdcParticipants, ok := cdcEvent.Fields[1].(cadence.Array)
	if !ok {
		return nil, invalidCadenceTypeError(
			"participants",
			cdcEvent.Fields[1],
			cadence.Array{},
		)
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

	// decode bytes using ccf
	payload, err := ccf.Decode(nil, event.Payload)
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
	identifierLists := make([]flow.IdentifierList, len(cdcClusters))
	for _, value := range cdcClusters {

		cdcCluster, ok := value.(cadence.Struct)
		if !ok {
			return nil, invalidCadenceTypeError("cluster", cdcCluster, cadence.Struct{})
		}

		expectedFields := 2
		if len(cdcCluster.Fields) < expectedFields {
			return nil, fmt.Errorf(
				"insufficient fields (%d < %d)",
				len(cdcCluster.Fields),
				expectedFields,
			)
		}

		// ensure cluster index is valid
		clusterIndex, ok := cdcCluster.Fields[0].(cadence.UInt16)
		if !ok {
			return nil, invalidCadenceTypeError(
				"clusterIndex",
				cdcCluster.Fields[0],
				cadence.UInt16(0),
			)
		}
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
		weightsByNodeID, ok := cdcCluster.Fields[1].(cadence.Dictionary)
		if !ok {
			return nil, invalidCadenceTypeError(
				"clusterWeights",
				cdcCluster.Fields[1],
				cadence.Dictionary{},
			)
		}

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
		cdcNodeInfoFields := cdcNodeInfoStruct.Fields

		expectedFields := 14
		if len(cdcNodeInfoFields) < expectedFields {
			return nil, fmt.Errorf(
				"insufficient fields (%d < %d)",
				len(cdcNodeInfoFields),
				expectedFields,
			)
		}

		// create and assign fields to identity from cadence Struct
		identity := new(flow.Identity)
		role, ok := cdcNodeInfoFields[1].(cadence.UInt8)
		if !ok {
			return nil, invalidCadenceTypeError(
				"nodeInfo.role",
				cdcNodeInfoFields[1],
				cadence.UInt8(0),
			)
		}
		identity.Role = flow.Role(role)
		if !identity.Role.Valid() {
			return nil, fmt.Errorf("invalid role %d", role)
		}

		address, ok := cdcNodeInfoFields[2].(cadence.String)
		if !ok {
			return nil, invalidCadenceTypeError(
				"nodeInfo.address",
				cdcNodeInfoFields[2],
				cadence.String(""),
			)
		}
		identity.Address = string(address)

		initialWeight, ok := cdcNodeInfoFields[13].(cadence.UInt64)
		if !ok {
			return nil, invalidCadenceTypeError(
				"nodeInfo.initialWeight",
				cdcNodeInfoFields[13],
				cadence.UInt64(0),
			)
		}
		identity.Weight = uint64(initialWeight)

		// convert nodeID string into identifier
		nodeIDHex, ok := cdcNodeInfoFields[0].(cadence.String)
		if !ok {
			return nil, invalidCadenceTypeError(
				"nodeInfo.id",
				cdcNodeInfoFields[0],
				cadence.String(""),
			)
		}
		identity.NodeID, err = flow.HexStringToIdentifier(string(nodeIDHex))
		if err != nil {
			return nil, fmt.Errorf("could not convert hex string to identifer: %w", err)
		}

		// parse to PublicKey the networking key hex string
		networkKeyHex, ok := cdcNodeInfoFields[3].(cadence.String)
		if !ok {
			return nil, invalidCadenceTypeError(
				"nodeInfo.networkKey",
				cdcNodeInfoFields[3],
				cadence.String(""),
			)
		}
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
		stakingKeyHex, ok := cdcNodeInfoFields[4].(cadence.String)
		if !ok {
			return nil, invalidCadenceTypeError(
				"nodeInfo.stakingKey",
				cdcNodeInfoFields[4],
				cadence.String(""),
			)
		}
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
		cdcClusterQCFields := cdcClusterQCStruct.Fields

		expectedFields := 4
		if len(cdcClusterQCFields) < expectedFields {
			return nil, fmt.Errorf(
				"insufficient fields (%d < %d)",
				len(cdcClusterQCFields),
				expectedFields,
			)
		}

		index, ok := cdcClusterQCFields[0].(cadence.UInt16)
		if !ok {
			return nil, invalidCadenceTypeError(
				"clusterQC.index",
				cdcClusterQCFields[0],
				cadence.UInt16(0),
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

		cdcVoterIDs, ok := cdcClusterQCFields[3].(cadence.Array)
		if !ok {
			return nil, invalidCadenceTypeError(
				"clusterQC.voterIDs",
				cdcClusterQCFields[2],
				cadence.Array{},
			)
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
		cdcRawVotes := cdcClusterQCFields[1].(cadence.Array)
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
		"VersionBeacon payload", payload, func(event cadence.Event) (
			flow.VersionBeacon,
			error,
		) {
			if len(event.Fields) != 2 {
				return flow.VersionBeacon{}, fmt.Errorf(
					"incorrect number of fields (%d != 2)",
					len(event.Fields),
				)
			}

			versionBoundaries, err := DecodeCadenceValue(
				".Fields[0]", event.Fields[0], convertVersionBoundaries,
			)
			if err != nil {
				return flow.VersionBeacon{}, err
			}

			sequence, err := DecodeCadenceValue(
				".Fields[1]", event.Fields[1], func(cadenceVal cadence.UInt64) (
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
				if len(structVal.Fields) < 2 {
					return flow.VersionBoundary{}, fmt.Errorf(
						"incorrect number of fields (%d != 2)",
						len(structVal.Fields),
					)
				}

				height, err := DecodeCadenceValue(
					".Fields[0]",
					structVal.Fields[0],
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
					".Fields[1]",
					structVal.Fields[1],
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
	if len(structVal.Fields) < 4 {
		return "", fmt.Errorf(
			"incorrect number of fields (%d != 4)",
			len(structVal.Fields),
		)
	}

	major, err := DecodeCadenceValue(
		".Fields[0]",
		structVal.Fields[0],
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
		".Fields[1]",
		structVal.Fields[1],
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
		".Fields[2]",
		structVal.Fields[2],
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
		".Fields[3]",
		structVal.Fields[3],
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

	version := fmt.Sprintf(
		"%d.%d.%d%s",
		major,
		minor,
		patch,
		preRelease,
	)
	_, err = semver.NewVersion(version)
	if err != nil {
		return "", fmt.Errorf(
			"invalid semver %s: %w",
			version,
			err,
		)
	}
	return version, nil

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
