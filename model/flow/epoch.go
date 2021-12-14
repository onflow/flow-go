package flow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/fxamacker/cbor/v2"
	"github.com/vmihailenco/msgpack/v4"

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

// EpochSetupRandomSourceLength is the required length of the random source
// included in an EpochSetup service event.
const EpochSetupRandomSourceLength = 16

// EpochSetup is a service event emitted when the network is ready to set up
// for the upcoming epoch. It contains the participants in the epoch, the
// length, the cluster assignment, and the seed for leader selection.
type EpochSetup struct {
	Counter            uint64         // the number of the epoch
	FirstView          uint64         // the first view of the epoch
	DKGPhase1FinalView uint64         // the final view of DKG phase 1
	DKGPhase2FinalView uint64         // the final view of DKG phase 2
	DKGPhase3FinalView uint64         // the final view of DKG phase 3
	FinalView          uint64         // the final view of the epoch
	Participants       IdentityList   // all participants of the epoch
	Assignments        AssignmentList // cluster assignment for the epoch
	RandomSource       []byte         // source of randomness for epoch-specific setup tasks
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

func (setup *EpochSetup) EqualTo(other *EpochSetup) bool {
	if setup.Counter != other.Counter {
		return false
	}
	if setup.FirstView != other.FirstView {
		return false
	}
	if setup.DKGPhase1FinalView != other.DKGPhase1FinalView {
		return false
	}
	if setup.DKGPhase2FinalView != other.DKGPhase2FinalView {
		return false
	}
	if setup.DKGPhase3FinalView != other.DKGPhase3FinalView {
		return false
	}
	if setup.FinalView != other.FinalView {
		return false
	}
	if !setup.Participants.EqualTo(other.Participants) {
		return false
	}
	if !setup.Assignments.EqualTo(other.Assignments) {
		return false
	}
	return bytes.Equal(setup.RandomSource, other.RandomSource)
}

// EpochCommit is a service event emitted when epoch setup has been completed.
// When an EpochCommit event is emitted, the network is ready to transition to
// the epoch.
type EpochCommit struct {
	Counter            uint64              // the number of the epoch
	ClusterQCs         []ClusterQCVoteData // quorum certificates for each cluster
	DKGGroupKey        crypto.PublicKey    // group key from DKG
	DKGParticipantKeys []crypto.PublicKey  // public keys for DKG participants
}

// ClusterQCVoteData represents the votes for a cluster quorum certificate, as
// gathered by the ClusterQC smart contract. It contains the aggregated
// signature over the root block for the cluster as well as the set of voters.
type ClusterQCVoteData struct {
	SigData  crypto.Signature // the aggregated signature over all the votes
	VoterIDs []Identifier     // the set of voters that contributed to the qc
}

func (c *ClusterQCVoteData) EqualTo(other *ClusterQCVoteData) bool {
	if len(c.VoterIDs) != len(other.VoterIDs) {
		return false
	}
	if !bytes.Equal(c.SigData, other.SigData) {
		return false
	}
	for i, v := range c.VoterIDs {
		if v != other.VoterIDs[i] {
			return false
		}
	}
	return true
}

// ClusterQCVoteDataFromQC converts a quorum certificate to the representation
// used by the smart contract, essentially discarding the block ID and view
// (which are protocol-defined given the EpochSetup event).
func ClusterQCVoteDataFromQC(qc *QuorumCertificate) ClusterQCVoteData {
	return ClusterQCVoteData{
		SigData:  qc.SigData,
		VoterIDs: qc.SignerIDs,
	}
}

func ClusterQCVoteDatasFromQCs(qcs []*QuorumCertificate) []ClusterQCVoteData {
	qcVotes := make([]ClusterQCVoteData, 0, len(qcs))
	for _, qc := range qcs {
		qcVotes = append(qcVotes, ClusterQCVoteDataFromQC(qc))
	}
	return qcVotes
}

func (commit *EpochCommit) ServiceEvent() ServiceEvent {
	return ServiceEvent{
		Type:  ServiceEventCommit,
		Event: commit,
	}
}

type encodableCommit struct {
	Counter            uint64
	ClusterQCs         []ClusterQCVoteData
	DKGGroupKey        encodable.RandomBeaconPubKey
	DKGParticipantKeys []encodable.RandomBeaconPubKey
}

func encodableFromCommit(commit *EpochCommit) encodableCommit {
	encKeys := make([]encodable.RandomBeaconPubKey, 0, len(commit.DKGParticipantKeys))
	for _, key := range commit.DKGParticipantKeys {
		encKeys = append(encKeys, encodable.RandomBeaconPubKey{PublicKey: key})
	}
	return encodableCommit{
		Counter:            commit.Counter,
		ClusterQCs:         commit.ClusterQCs,
		DKGGroupKey:        encodable.RandomBeaconPubKey{PublicKey: commit.DKGGroupKey},
		DKGParticipantKeys: encKeys,
	}
}

func commitFromEncodable(enc encodableCommit) EpochCommit {
	dkgKeys := make([]crypto.PublicKey, 0, len(enc.DKGParticipantKeys))
	for _, key := range enc.DKGParticipantKeys {
		dkgKeys = append(dkgKeys, key.PublicKey)
	}
	return EpochCommit{
		Counter:            enc.Counter,
		ClusterQCs:         enc.ClusterQCs,
		DKGGroupKey:        enc.DKGGroupKey.PublicKey,
		DKGParticipantKeys: dkgKeys,
	}
}

func (commit EpochCommit) MarshalJSON() ([]byte, error) {
	return json.Marshal(encodableFromCommit(&commit))
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

func (commit *EpochCommit) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(encodableFromCommit(commit))
}

func (commit *EpochCommit) UnmarshalCBOR(b []byte) error {
	var enc encodableCommit
	err := cbor.Unmarshal(b, &enc)
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
// NOTE: DecodeRLP is not needed, as this is only used for hashing.
func (commit *EpochCommit) EncodeRLP(w io.Writer) error {
	rlpEncodable := struct {
		Counter            uint64
		ClusterQCs         []ClusterQCVoteData
		DKGGroupKey        []byte
		DKGParticipantKeys [][]byte
	}{
		Counter:            commit.Counter,
		ClusterQCs:         commit.ClusterQCs,
		DKGGroupKey:        commit.DKGGroupKey.Encode(),
		DKGParticipantKeys: make([][]byte, 0, len(commit.DKGParticipantKeys)),
	}
	for _, key := range commit.DKGParticipantKeys {
		rlpEncodable.DKGParticipantKeys = append(rlpEncodable.DKGParticipantKeys, key.Encode())
	}

	return rlp.Encode(w, rlpEncodable)
}

// ID returns the hash of the event contents.
func (commit *EpochCommit) ID() Identifier {
	return MakeID(commit)
}

func (commit *EpochCommit) EqualTo(other *EpochCommit) bool {
	if commit.Counter != other.Counter {
		return false
	}
	if len(commit.ClusterQCs) != len(other.ClusterQCs) {
		return false
	}
	for i, qc := range commit.ClusterQCs {
		if !qc.EqualTo(&other.ClusterQCs[i]) {
			return false
		}
	}
	if (commit.DKGGroupKey == nil && other.DKGGroupKey != nil) ||
		(commit.DKGGroupKey != nil && other.DKGGroupKey == nil) {
		return false
	}
	if commit.DKGGroupKey != nil && other.DKGGroupKey != nil && !commit.DKGGroupKey.Equals(other.DKGGroupKey) {
		return false
	}
	if len(commit.DKGParticipantKeys) != len(other.DKGParticipantKeys) {
		return false
	}

	for i, key := range commit.DKGParticipantKeys {
		if !key.Equals(other.DKGParticipantKeys[i]) {
			return false
		}
	}

	return true
}

// ToDKGParticipantLookup constructs a DKG participant lookup from an identity
// list and a key list. The identity list must be EXACTLY the same (order and
// contents) as that used when initializing the corresponding DKG instance.
func ToDKGParticipantLookup(participants IdentityList, keys []crypto.PublicKey) (map[Identifier]DKGParticipant, error) {
	if len(participants) != len(keys) {
		return nil, fmt.Errorf("participant list (len=%d) does not match key list (len=%d)", len(participants), len(keys))
	}

	lookup := make(map[Identifier]DKGParticipant, len(participants))
	for i := 0; i < len(participants); i++ {
		part := participants[i]
		key := keys[i]
		lookup[part.NodeID] = DKGParticipant{
			Index:    uint(i),
			KeyShare: key,
		}
	}
	return lookup, nil
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

func (part DKGParticipant) MarshalCBOR() ([]byte, error) {
	enc := encodableFromDKGParticipant(part)
	return cbor.Marshal(enc)
}

func (part *DKGParticipant) UnmarshalCBOR(b []byte) error {
	var enc encodableDKGParticipant
	err := cbor.Unmarshal(b, &enc)
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
	PreviousEpoch EventIDs // EpochSetup and EpochCommit events for the previous epoch
	CurrentEpoch  EventIDs // EpochSetup and EpochCommit events for the current epoch
	NextEpoch     EventIDs // EpochSetup and EpochCommit events for the next epoch
}

// Copy returns a copy of the epoch status.
func (es *EpochStatus) Copy() *EpochStatus {
	return &EpochStatus{
		PreviousEpoch: es.PreviousEpoch,
		CurrentEpoch:  es.CurrentEpoch,
		NextEpoch:     es.NextEpoch,
	}
}

// EventIDs is a container for IDs of epoch service events.
type EventIDs struct {
	// SetupID is the ID of the EpochSetup event for the respective Epoch
	SetupID Identifier
	// CommitID is the ID of the EpochCommit event for the respective Epoch
	CommitID Identifier
}

func NewEpochStatus(previousSetup, previousCommit, currentSetup, currentCommit, nextSetup, nextCommit Identifier) (*EpochStatus, error) {
	status := &EpochStatus{
		PreviousEpoch: EventIDs{
			SetupID:  previousSetup,
			CommitID: previousCommit,
		},
		CurrentEpoch: EventIDs{
			SetupID:  currentSetup,
			CommitID: currentCommit,
		},
		NextEpoch: EventIDs{
			SetupID:  nextSetup,
			CommitID: nextCommit,
		},
	}

	err := status.Check()
	if err != nil {
		return nil, err
	}
	return status, nil
}

// Check checks that the status is well-formed, returning an error if it is not.
func (es *EpochStatus) Check() error {

	if es == nil {
		return fmt.Errorf("nil epoch status")
	}
	// must reference either both or neither event IDs for previous epoch
	if (es.PreviousEpoch.SetupID == ZeroID) != (es.PreviousEpoch.CommitID == ZeroID) {
		return fmt.Errorf("epoch status with only setup or only commit service event")
	}
	// must reference event IDs for current epoch
	if es.CurrentEpoch.SetupID == ZeroID || es.CurrentEpoch.CommitID == ZeroID {
		return fmt.Errorf("epoch status with empty current epoch service events")
	}
	// must not reference a commit without a setup
	if es.NextEpoch.SetupID == ZeroID && es.NextEpoch.CommitID != ZeroID {
		return fmt.Errorf("epoch status with commit but no setup service event")
	}
	return nil
}

// Phase returns the phase for the CURRENT epoch, given this epoch status.
func (es *EpochStatus) Phase() (EpochPhase, error) {

	err := es.Check()
	if err != nil {
		return EpochPhaseUndefined, err
	}
	if es.NextEpoch.SetupID == ZeroID {
		return EpochPhaseStaking, nil
	}
	if es.NextEpoch.CommitID == ZeroID {
		return EpochPhaseSetup, nil
	}
	return EpochPhaseCommitted, nil
}

func (es *EpochStatus) HasPrevious() bool {
	return es.PreviousEpoch.SetupID != ZeroID && es.PreviousEpoch.CommitID != ZeroID
}
