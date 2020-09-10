package flow

import (
	"encoding/binary"
	"encoding/json"

	"github.com/vmihailenco/msgpack"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encodable"
)

// EpochPhase represents a phase of the Epoch Preparation Protocol. The phase
// of an epoch is resolved based on a block reference and is fork-dependent.
// An epoch begins in the staking phase, then transitions to the setup phase in
// the block containing the EpochSetup service event, then to the committed
// phase in the block containing the EpochCommit service event.
type EpochPhase int

const (
	EpochPhaseUnknown EpochPhase = iota
	EpochPhaseStaking
	EpochPhaseSetup
	EpochPhaseCommitted
)

func (p EpochPhase) String() string {
	return [...]string{
		"EpochPhaseUnknown",
		"EpochStakingPhase",
		"EpochSetupPhase",
		"EpochCommittedPhase",
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

// ID returns a unique ID for the epoch, based on the counter. This
// is used as a work-around for the current caching layer, which only
// supports flow entities keyed by ID for now.
func (setup *EpochSetup) ID() Identifier {
	var commitID Identifier
	binary.LittleEndian.PutUint64(commitID[:], setup.Counter)
	return commitID
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

// ID returns a unique ID for the epoch, based on the counter. This
// is used as a work-around for the current caching layer, which only
// suports flow entities keyed by ID for now.
func (commit *EpochCommit) ID() Identifier {
	var commitID Identifier
	binary.LittleEndian.PutUint64(commitID[:], commit.Counter)
	return commitID
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

// EpochStatus represents the status of the current and next epoch with respect
// to a reference block. Concretely, it contains the IDs for all relevant
// service events emitted as of the reference block. Events not yet emitted are
// represented by ZeroID.
type EpochStatus struct {
	CurrentEpoch EventIDs // Epoch Preparation Events for the current Epoch
	NextEpoch    EventIDs // Epoch Preparation Events for the next Epoch
}

func NewEpochState(currentSetup, currentCommit, nextSetup, nextCommit Identifier) *EpochStatus {
	return &EpochStatus{
		CurrentEpoch: EventIDs{
			Setup:  currentSetup,
			Commit: currentCommit,
		},
		NextEpoch: EventIDs{
			Setup:  nextSetup,
			Commit: nextCommit,
		},
	}
}

// Phase returns the phase for the CURRENT epoch, given this epoch status.
func (es *EpochStatus) Phase() EpochPhase {

	// invalid for any current epoch events to be empty
	if es.CurrentEpoch.Setup == ZeroID || es.CurrentEpoch.Commit == ZeroID {
		return EpochPhaseUnknown
	}
	// invalid for commit to have been emitted before setup
	if es.NextEpoch.Setup == ZeroID && es.NextEpoch.Commit != ZeroID {
		return EpochPhaseUnknown
	}

	if es.NextEpoch.Setup == ZeroID {
		return EpochPhaseStaking
	}
	if es.NextEpoch.Commit == ZeroID {
		return EpochPhaseSetup
	}
	return EpochPhaseCommitted
}

type EventIDs struct {
	// SetupEventID is the ID of the EpochSetup event for the respective Epoch
	Setup Identifier

	// CommitEventID is the ID of the EpochCommit event for the respective Epoch
	Commit Identifier
}
