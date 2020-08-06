package flow

import (
	"encoding/binary"
	"encoding/json"

	hotstuff "github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encodable"
)

// TODO docs
type EpochSetup struct {
	Counter      uint64
	FinalView    uint64
	Participants IdentityList
	Assignments  AssignmentList
	Seed         []byte
}

func (setup *EpochSetup) ToServiceEvent() *ServiceEvent {
	return &ServiceEvent{
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

// TODO docs
type EpochCommit struct {
	Counter         uint64
	ClusterQCs      []*hotstuff.QuorumCertificate
	DKGGroupKey     crypto.PublicKey
	DKGParticipants map[Identifier]DKGParticipant
}

func (commit *EpochCommit) ToServiceEvent() *ServiceEvent {
	return &ServiceEvent{
		Type:  ServiceEventCommit,
		Event: commit,
	}
}

type encodableCommit struct {
	Counter         uint64
	ClusterQCs      []*hotstuff.QuorumCertificate
	DKGGroupKey     encodable.RandomBeaconPubKey
	DKGParticipants map[Identifier]DKGParticipant
}

func (commit *EpochCommit) MarshalJSON() ([]byte, error) {
	enc := encodableCommit{
		Counter:         commit.Counter,
		ClusterQCs:      commit.ClusterQCs,
		DKGGroupKey:     encodable.RandomBeaconPubKey{commit.DKGGroupKey},
		DKGParticipants: commit.DKGParticipants,
	}
	return json.Marshal(enc)
}

func (commit *EpochCommit) UnmarshalJSON(b []byte) error {
	var enc encodableCommit
	err := json.Unmarshal(b, &enc)
	if err != nil {
		return err
	}

	*commit = EpochCommit{
		Counter:         enc.Counter,
		ClusterQCs:      enc.ClusterQCs,
		DKGGroupKey:     enc.DKGGroupKey.PublicKey,
		DKGParticipants: enc.DKGParticipants,
	}
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

func (part DKGParticipant) MarshalJSON() ([]byte, error) {
	enc := encodableDKGParticipant{
		Index:    part.Index,
		KeyShare: encodable.RandomBeaconPubKey{part.KeyShare},
	}
	return json.Marshal(enc)
}

func (part *DKGParticipant) UnmarshalJSON(b []byte) error {
	var enc encodableDKGParticipant
	err := json.Unmarshal(b, &enc)
	if err != nil {
		return err
	}

	*part = DKGParticipant{
		Index:    enc.Index,
		KeyShare: enc.KeyShare.PublicKey,
	}
	return nil
}
