package flow

import (
	"encoding/json"
	"io"
	"sort"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encodable"
)

// EpochPhase represents a phase of the Epoch Preparation Protocol. The phase
// of an epoch is resolved based on a block reference and is fork-dependent.
// An epoch begins in the staking phase, then transitions to the setup phase in
// the block containing the EpochSetup service event, then to the committed
// phase in the block containing the EpochCommit service event.
// |<--  EpochPhaseStaking -->|<-- EpochPhaseSetup -->|<-- EpochPhaseCommitted -->|<-- EpochPhaseStaking -->...
// |<------------------------------- Epoch N ------------------------------------>|<-- Epoch N + 1 --...
type EpochPhase int

const (
	EpochPhaseUndefined EpochPhase = iota
	EpochPhaseStaking
	EpochPhaseSetup
	EpochPhaseCommitted
)

func (p EpochPhase) String() string {
	return [...]string{
		"EpochPhaseUndefined",
		"EpochPhaseStaking",
		"EpochPhaseSetup",
		"EpochPhaseCommitted",
	}[p]
}

// EpochSetup is a service event emitted when the network is ready to set up
// for the upcoming epoch. It contains the participants in the epoch, the
// length, the cluster assignment, and the seed for leader selection.
type EpochSetup struct {
	Counter      uint64         // the number of the epoch
	FinalView    uint64         // the final view of the epoch
	Participants IdentityList   // all participants of the epoch
	Assignments  AssignmentList // cluster assignment for the epoch
	RandomSource []byte         // source of randomness for epoch-specific setup tasks
}

func (setup *EpochSetup) ServiceEvent() ServiceEvent {
	return ServiceEvent{
		Type:  ServiceEventSetup,
		Event: setup,
	}
}

// ID returns the hash of the event contents.
func (setup *EpochSetup) ID() Identifier {
	return MakeID(setup)
}

// EpochCommit is a service event emitted when epoch setup has been completed.
// When an EpochCommit event is emitted, the network is ready to transition to
// the epoch.
type EpochCommit struct {
	Counter         uint64                        // the number of the epoch
	ClusterQCs      []*QuorumCertificate          // quorum certificates for each cluster
	DKGGroupKey     crypto.PublicKey              // group key from DKG
	DKGParticipants map[Identifier]DKGParticipant // public keys for DKG participants
}

func (commit *EpochCommit) ServiceEvent() ServiceEvent {
	return ServiceEvent{
		Type:  ServiceEventCommit,
		Event: commit,
	}
}

type encodableCommit struct {
	Counter         uint64
	ClusterQCs      []*QuorumCertificate
	DKGGroupKey     encodable.RandomBeaconPubKey
	DKGParticipants map[Identifier]DKGParticipant
}

func encodableFromCommit(commit *EpochCommit) encodableCommit {
	return encodableCommit{
		Counter:         commit.Counter,
		ClusterQCs:      commit.ClusterQCs,
		DKGGroupKey:     encodable.RandomBeaconPubKey{PublicKey: commit.DKGGroupKey},
		DKGParticipants: commit.DKGParticipants,
	}
}

func commitFromEncodable(enc encodableCommit) EpochCommit {
	return EpochCommit{
		Counter:         enc.Counter,
		ClusterQCs:      enc.ClusterQCs,
		DKGGroupKey:     enc.DKGGroupKey.PublicKey,
		DKGParticipants: enc.DKGParticipants,
	}
}

func (commit *EpochCommit) MarshalJSON() ([]byte, error) {
	return json.Marshal(encodableFromCommit(commit))
}

func (commit *EpochCommit) UnmarshalJSON(b []byte) error {
	var enc encodableCommit
	err := json.Unmarshal(b, &enc)
	if err != nil {
		return err
	}

	*commit = commitFromEncodable(enc)
	return nil
}

func (commit *EpochCommit) MarshalMsgpack() ([]byte, error) {
	return msgpack.Marshal(encodableFromCommit(commit))
}

func (commit *EpochCommit) UnmarshalMsgpack(b []byte) error {
	var enc encodableCommit
	err := msgpack.Unmarshal(b, &enc)
	if err != nil {
		return err
	}
	*commit = commitFromEncodable(enc)
	return nil
}

// EncodeRLP encodes the commit as RLP. The RLP encoding needs to be handled
// differently from JSON/msgpack, because it does not handle custom encoders
// within map types.
func (commit *EpochCommit) EncodeRLP(w io.Writer) error {
	rlpEncodable := struct {
		Counter         uint64
		ClusterQCs      []*QuorumCertificate
		DKGGroupKey     []byte
		DKGParticipants []struct {
			NodeID []byte
			Part   encodableDKGParticipant
		}
	}{
		Counter:     commit.Counter,
		ClusterQCs:  commit.ClusterQCs,
		DKGGroupKey: commit.DKGGroupKey.Encode(),
	}
	for nodeID, part := range commit.DKGParticipants {
		// must copy the node ID, since the loop variable references the same
		// backing memory for each iteration
		nodeIDRaw := make([]byte, len(nodeID))
		copy(nodeIDRaw, nodeID[:])

		rlpEncodable.DKGParticipants = append(rlpEncodable.DKGParticipants, struct {
			NodeID []byte
			Part   encodableDKGParticipant
		}{
			NodeID: nodeIDRaw,
			Part:   encodableFromDKGParticipant(part),
		})
	}

	// sort to ensure consistent ordering prior to encoding
	sort.Slice(rlpEncodable.DKGParticipants, func(i, j int) bool {
		return rlpEncodable.DKGParticipants[i].Part.Index < rlpEncodable.DKGParticipants[j].Part.Index
	})

	return rlp.Encode(w, rlpEncodable)
}

// ID returns the hash of the event contents.
func (commit *EpochCommit) ID() Identifier {
	return MakeID(commit)
}

type DKGParticipant struct {
	Index    uint
	KeyShare crypto.PublicKey
}

type encodableDKGParticipant struct {
	Index    uint
	KeyShare encodable.RandomBeaconPubKey
}

func encodableFromDKGParticipant(part DKGParticipant) encodableDKGParticipant {
	return encodableDKGParticipant{
		Index:    part.Index,
		KeyShare: encodable.RandomBeaconPubKey{PublicKey: part.KeyShare},
	}
}

func dkgParticipantFromEncodable(enc encodableDKGParticipant) DKGParticipant {
	return DKGParticipant{
		Index:    enc.Index,
		KeyShare: enc.KeyShare.PublicKey,
	}
}

func (part DKGParticipant) MarshalJSON() ([]byte, error) {
	enc := encodableFromDKGParticipant(part)
	return json.Marshal(enc)
}

func (part *DKGParticipant) UnmarshalJSON(b []byte) error {
	var enc encodableDKGParticipant
	err := json.Unmarshal(b, &enc)
	if err != nil {
		return err
	}

	*part = dkgParticipantFromEncodable(enc)
	return nil
}

func (part DKGParticipant) MarshalMsgpack() ([]byte, error) {
	return msgpack.Marshal(encodableFromDKGParticipant(part))
}

func (part *DKGParticipant) UnmarshalMsgpack(b []byte) error {
	var enc encodableDKGParticipant
	err := msgpack.Unmarshal(b, &enc)
	if err != nil {
		return err
	}
	*part = dkgParticipantFromEncodable(enc)
	return nil
}

func (part DKGParticipant) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, encodableFromDKGParticipant(part))
}

// EpochStatus represents the status of the current and next epoch with respect
// to a reference block. Concretely, it contains the IDs for all relevant
// service events emitted as of the reference block. Events not yet emitted are
// represented by ZeroID.
type EpochStatus struct {
	CurrentEpoch EventIDs // Epoch Preparation Events for the current Epoch
	NextEpoch    EventIDs // Epoch Preparation Events for the next Epoch
}

type EventIDs struct {
	// SetupID is the ID of the EpochSetup event for the respective Epoch
	SetupID Identifier

	// CommitID is the ID of the EpochCommit event for the respective Epoch
	CommitID Identifier
}

func NewEpochStatus(currentSetup, currentCommit, nextSetup, nextCommit Identifier) *EpochStatus {
	return &EpochStatus{
		CurrentEpoch: EventIDs{
			SetupID:  currentSetup,
			CommitID: currentCommit,
		},
		NextEpoch: EventIDs{
			SetupID:  nextSetup,
			CommitID: nextCommit,
		},
	}
}

// Valid returns true if the status is well-formed.
func (es *EpochStatus) Valid() bool {

	if es == nil {
		return false
	}
	// must reference event IDs for current epoch
	if es.CurrentEpoch.SetupID == ZeroID || es.CurrentEpoch.CommitID == ZeroID {
		return false
	}
	// must not reference a commit without a setup
	if es.NextEpoch.SetupID == ZeroID && es.NextEpoch.CommitID != ZeroID {
		return false
	}
	return true
}

// Phase returns the phase for the CURRENT epoch, given this epoch status.
func (es *EpochStatus) Phase() EpochPhase {

	if !es.Valid() {
		return EpochPhaseUndefined
	}
	if es.NextEpoch.SetupID == ZeroID {
		return EpochPhaseStaking
	}
	if es.NextEpoch.CommitID == ZeroID {
		return EpochPhaseSetup
	}
	return EpochPhaseCommitted
}
