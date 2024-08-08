package flow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/crypto"
	"github.com/vmihailenco/msgpack/v4"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"github.com/onflow/flow-go/model/encodable"
)

// EpochPhase represents a phase of the Epoch Preparation Protocol.
// The phase of an epoch is resolved based on a block reference and is fork-dependent.
// During normal operations, each Epoch transitions through the phases:
//
//	║                                       Epoch N
//	║       ╭─────────────────────────────────┴─────────────────────────────────╮
//	║   finalize view            EpochSetup            EpochCommit
//	║     in epoch N            service event         service event
//	║        ⇣                       ⇣                     ⇣
//	║        ┌─────────────────┐     ┌───────────────┐     ┌───────────────────┐
//	║        │EpochPhaseStaking├─────►EpochPhaseSetup├─────►EpochPhaseCommitted├ ┄>
//	║        └─────────────────┘     └───────────────┘     └───────────────────┘
//	║        ⇣                       ⇣                     ⇣
//	║   EpochTransition     EpochSetupPhaseStarted    EpochCommittedPhaseStarted
//	║    Notification            Notification               Notification
//
// However, if the Protocol State encounters any unexpected epoch service events, or the subsequent epoch
// fails to be committed by the `EpochCommitSafetyThreshold`, then we enter Epoch Fallback Mode [EFM].
// Depending on whether the subsequent epoch has already been committed, the EFM progress differs slightly.
// In a nutshell, we always enter the _latest_ epoch already committed on the happy path (if there is any)
// and then follow the fallback protocol.
//
// SCENARIO A: the future Epoch N is already committed, when we enter EFM
//
//	║      Epoch N-1                            Epoch N
//	║   ···──┴─────────────────────────╮ ╭─────────────┴───────────────────────────────────────────────╮
//	║      invalid service                finalize view                   EpochRecover
//	║            event                    in epoch N                      service event
//	║              ⇣                      ⇣                    ┊          ⇣
//	║     ┌──────────────────────────┐    ┌────────────────────┊────┐     ┌───────────────────────────┐
//	║     │   EpochPhaseCommitted    ├────►    EpochPhaseFallback   ├─────►    EpochPhaseCommitted    ├ ┄>
//	║     └──────────────────────────┘    └────────────────────┊────┘     └───────────────────────────┘
//	║              ⇣                      ⇣                    ┊          ⇣
//	║   EpochFallbackModeTriggered     EpochTransition   EpochExtended*   EpochFallbackModeExited
//	║          Notification             Notification      Notification    + EpochCommittedPhaseStarted Notifications
//	║              ┆                                                      ┆
//	║              ╰┄┄┄┄┄┄┄┄┄┄ EpochFallbackTriggered is true ┄┄┄┄┄┄┄┄┄┄┄┄╯
//
// With 'EpochExtended*' we denote that there can be zero, one, or more Epoch Extension (depending on when
// we receive a valid EpochRecover service event.
//
// SCENARIO B: we are in Epoch N without any subsequent epoch being committed when entering EFM
//
//	║                         Epoch N
//	║ ···────────────────────────┴───────────────────────────────────────────────────────────────╮
//	║              invalid service event or                         EpochRecover
//	║         EpochCommitSafetyThreshold reached                    service event
//	║                           ⇣                      ┊            ⇣
//	║  ┌────────────────────┐   ┌──────────────────────┊──────┐     ┌───────────────────────────┐
//	║  │ EpochPhaseStaking  │   │     EpochPhaseFallback      │     │   EpochPhaseCommitted     │
//	║  │ or EpochPhaseSetup ├───►                      ┊      ├─────►                           ├ ┄>
//	║  └────────────────────┘   └──────────────────────┊──────┘     └───────────────────────────┘
//	║                           ⇣                      ┊            ⇣
//	║            EpochFallbackModeTriggered     EpochExtended*      EpochFallbackModeExited
//	║                     Notification           Notification       + EpochCommittedPhaseStarted Notifications
//	║                           ┆                                   ┆
//	║                           ╰┄┄ EpochFallbackTriggered true ┄┄┄┄╯
//
// A state machine diagram containing all possible phase transitions is below:
//
//	         ┌──────────────────────────────────────────────────────────┐
//	┌────────▼────────┐     ┌───────────────┐     ┌───────────────────┐ │
//	│EpochPhaseStaking├─────►EpochPhaseSetup├─────►EpochPhaseCommitted├─┘
//	└────────┬────────┘     └───────────┬───┘     └───┬──────────▲────┘
//	         │                        ┌─▼─────────────▼──┐       │
//	         └────────────────────────►EpochPhaseFallback├───────┘
//	                                  └──────────────────┘
type EpochPhase int

const (
	EpochPhaseUndefined EpochPhase = iota
	EpochPhaseStaking
	EpochPhaseSetup
	EpochPhaseCommitted
	EpochPhaseFallback
)

func (p EpochPhase) String() string {
	return [...]string{
		"EpochPhaseUndefined",
		"EpochPhaseStaking",
		"EpochPhaseSetup",
		"EpochPhaseCommitted",
		"EpochPhaseFallback",
	}[p]
}

func GetEpochPhase(phase string) EpochPhase {
	phases := []EpochPhase{
		EpochPhaseUndefined,
		EpochPhaseStaking,
		EpochPhaseSetup,
		EpochPhaseCommitted,
		EpochPhaseFallback,
	}
	for _, p := range phases {
		if p.String() == phase {
			return p
		}
	}

	return EpochPhaseUndefined
}

// EpochSetupRandomSourceLength is the required length of the random source
// included in an EpochSetup service event.
const EpochSetupRandomSourceLength = 16

// EpochSetup is a service event emitted when the network is ready to set up
// for the upcoming epoch. It contains the participants in the epoch, the
// length, the cluster assignment, and the seed for leader selection.
// EpochSetup is a service event emitted when the preparation process for the next epoch begins.
// EpochSetup events must:
//   - be emitted exactly once per epoch before the corresponding EpochCommit event
//   - be emitted prior to the epoch commitment deadline (defined by EpochCommitSafetyThreshold)
//
// If either of the above constraints are not met, the service event will be rejected and Epoch Fallback Mode [EFM] will be triggered.
//
// When an EpochSetup event is accepted and incorporated into the Protocol State, this triggers the
// Distributed Key Generation [DKG] and cluster QC voting process for the next epoch.
// It also causes the current epoch to enter the EpochPhaseSetup phase.
type EpochSetup struct {
	Counter            uint64               // the number of the epoch being setup (current+1)
	FirstView          uint64               // the first view of the epoch being setup
	DKGPhase1FinalView uint64               // the final view of DKG phase 1
	DKGPhase2FinalView uint64               // the final view of DKG phase 2
	DKGPhase3FinalView uint64               // the final view of DKG phase 3
	FinalView          uint64               // the final view of the epoch
	Participants       IdentitySkeletonList // all participants of the epoch in canonical order
	Assignments        AssignmentList       // cluster assignment for the epoch
	RandomSource       []byte               // source of randomness for epoch-specific setup tasks
	TargetDuration     uint64               // desired real-world duration for the epoch [seconds]
	TargetEndTime      uint64               // desired real-world end time for the epoch in UNIX time [seconds]
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
	if setup.TargetDuration != other.TargetDuration {
		return false
	}
	if setup.TargetEndTime != other.TargetEndTime {
		return false
	}
	if !IdentitySkeletonListEqualTo(setup.Participants, other.Participants) {
		return false
	}
	if !setup.Assignments.EqualTo(other.Assignments) {
		return false
	}
	return bytes.Equal(setup.RandomSource, other.RandomSource)
}

// EpochRecover service event is emitted when network is in Epoch Fallback Mode(EFM) in an attempt to return to happy path.
// It contains data from EpochSetup, and EpochCommit events to so replicas can create a committed epoch from which they
// can continue operating on the happy path.
type EpochRecover struct {
	EpochSetup
	EpochCommit
}

func (er *EpochRecover) ServiceEvent() ServiceEvent {
	return ServiceEvent{
		Type:  ServiceEventRecover,
		Event: er,
	}
}

// ID returns the hash of the event contents.
func (er *EpochRecover) ID() Identifier {
	return MakeID(er)
}

func (er *EpochRecover) EqualTo(other *EpochRecover) bool {
	if !er.EpochSetup.EqualTo(&other.EpochSetup) {
		return false
	}
	if !er.EpochCommit.EqualTo(&other.EpochCommit) {
		return false
	}
	return true
}

// EpochCommit is a service event emitted when the preparation process for the next epoch is complete.
// EpochCommit events must:
//   - be emitted exactly once per epoch after the corresponding EpochSetup event
//   - be emitted prior to the epoch commitment deadline (defined by EpochCommitSafetyThreshold)
//
// If either of the above constraints are not met, the service event will be rejected and Epoch Fallback Mode [EFM] will be triggered.
//
// When an EpochCommit event is accepted and incorporated into the Protocol State, this guarantees that
// the network will proceed through that epoch's defined view range with its defined committee. It also
// causes the current epoch to enter the EpochPhaseCommitted phase.
//
// TERMINOLOGY NOTE: In the context of the Epoch Preparation Protocol and the EpochCommit event,
// artifacts produced by the DKG are referred to with the "DKG" prefix (for example, DKGGroupKey).
// These artifacts are *produced by* the DKG, but used for the Random Beacon. As such, other
// components refer to these same artifacts with the "RandomBeacon" prefix.
type EpochCommit struct {
	// Counter is the epoch counter of the epoch being committed
	Counter uint64
	// ClusterQCs is an ordered list of root quorum certificates, one per cluster.
	// EpochCommit.ClustersQCs[i] is the QC for EpochSetup.Assignments[i]
	ClusterQCs []ClusterQCVoteData
	// DKGGroupKey is the group public key produced by the DKG associated with this epoch.
	// It is used to verify Random Beacon signatures for the epoch with counter, Counter.
	DKGGroupKey crypto.PublicKey
	// DKGParticipantKeys is a list of public keys, one per DKG participant, ordered by Random Beacon index.
	// This list is the output of the DKG associated with this epoch.
	// It is used to verify Random Beacon signatures for the epoch with counter, Counter.
	// CAUTION: This list may include keys for nodes which do not exist in the consensus committee
	//          and may NOT include keys for all nodes in the consensus committee.
	DKGParticipantKeys []crypto.PublicKey
	// DKGIndexMap is always nil and is not used. This field exists to avoid data-model changes in a future version.
	// Deprecated: This field is always nil and should not be used (it isn't really deprecated -- "pre-un-deprecated", maybe --
	// but marking it as such makes Go tooling flag it in a way that is useful for this circumstance)
	//
	// TODO(EFM, #6214): Parse this field from service event and make use of it.
	//                   Here is what the godoc should look like once we do that:
	// DKGIndexMap is a mapping from node identifier to Random Beacon index.
	// It has the following invariants:
	//   - len(DKGParticipantKeys) == len(DKGIndexMap)
	//   - DKGIndexMap values form the set {0, 1, ..., n-1} where n=len(DKGParticipantKeys)
	// CAUTION: This mapping may include identifiers for nodes which do not exist in the consensus committee
	//          and may NOT include identifiers for all nodes in the consensus committee.
	//
	DKGIndexMap map[Identifier]int
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
func ClusterQCVoteDataFromQC(qc *QuorumCertificateWithSignerIDs) ClusterQCVoteData {
	return ClusterQCVoteData{
		SigData:  qc.SigData,
		VoterIDs: qc.SignerIDs,
	}
}

func ClusterQCVoteDatasFromQCs(qcs []*QuorumCertificateWithSignerIDs) []ClusterQCVoteData {
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

	if !slices.EqualFunc(commit.ClusterQCs, other.ClusterQCs, func(qc1 ClusterQCVoteData, qc2 ClusterQCVoteData) bool {
		return qc1.EqualTo(&qc2)
	}) {
		return false
	}

	if (commit.DKGGroupKey == nil && other.DKGGroupKey != nil) ||
		(commit.DKGGroupKey != nil && other.DKGGroupKey == nil) {
		return false
	}
	if commit.DKGGroupKey != nil && other.DKGGroupKey != nil && !commit.DKGGroupKey.Equals(other.DKGGroupKey) {
		return false
	}

	if !slices.EqualFunc(commit.DKGParticipantKeys, other.DKGParticipantKeys, func(k1 crypto.PublicKey, k2 crypto.PublicKey) bool {
		return k1.Equals(k2)
	}) {
		return false
	}

	if !maps.Equal(commit.DKGIndexMap, other.DKGIndexMap) {
		return false
	}

	return true
}

// EjectNode is a service event emitted when a node has to be ejected from the network.
// Dynamic Protocol State listens to this event and updates the identity table accordingly.
// It contains a single field which is the identifier of the node being ejected.
type EjectNode struct {
	NodeID Identifier
}

// EqualTo returns true if the two events are equivalent.
func (e *EjectNode) EqualTo(other *EjectNode) bool {
	return e.NodeID == other.NodeID
}

// ServiceEvent returns the event as a generic ServiceEvent type.
func (e *EjectNode) ServiceEvent() ServiceEvent {
	return ServiceEvent{
		Type:  ServiceEventEjectNode,
		Event: e,
	}
}

// ToDKGParticipantLookup constructs a DKG participant lookup from an identity
// list and a key list. The identity list must be EXACTLY the same (order and
// contents) as that used when initializing the corresponding DKG instance.
// TODO(EFM, #6214): Once DKGIndexMap is populated we can remove this and use EpochCommit directly
func ToDKGParticipantLookup(participants IdentitySkeletonList, keys []crypto.PublicKey) (map[Identifier]DKGParticipant, error) {
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

// EventIDs is a container for IDs of epoch service events.
type EventIDs struct {
	// SetupID is the ID of the EpochSetup event for the respective Epoch
	SetupID Identifier
	// CommitID is the ID of the EpochCommit event for the respective Epoch
	CommitID Identifier
}

// ID returns hash of the event IDs.
func (e *EventIDs) ID() Identifier {
	return MakeID(e)
}
