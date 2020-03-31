// Package bootstrap defines canonical models and encoding for bootstrapping.
package bootstrap

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
)

type GenesisBlock flow.Block

func (b *GenesisBlock) Encode() ([]byte, error) {
	return encoding.DefaultEncoder.Encode(b)
}

func DecodeGenesisBlock(raw []byte, block *GenesisBlock) error {
	return encoding.DefaultEncoder.Decode(raw, block)
}

type GenesisQC model.QuorumCertificate

func (qc *GenesisQC) Encode() ([]byte, error) {
	return encoding.DefaultEncoder.Encode(qc)
}

func DecodeGenesisQC(raw []byte, qc *GenesisQC) error {
	return encoding.DefaultEncoder.Decode(raw, qc)
}

type DKGParticipant struct {
	ID          flow.Identifier
	PubKeyShare EncodableRandomBeaconPubKey
	GroupIndex  int
}

type DKGPubData struct {
	GroupPubKey  EncodableRandomBeaconPubKey
	Participants []DKGParticipant
}

func (qc *DKGPubData) Encode() ([]byte, error) {
	return encoding.DefaultEncoder.Encode(qc)
}

func DecodeDKGPubData(raw []byte, dkgPubData *DKGPubData) error {
	return encoding.DefaultEncoder.Decode(raw, dkgPubData)
}

func (data *DKGPubData) ForHotStuff() *hotstuff.DKGPublicData {

	participantLookup := make(map[flow.Identifier]*hotstuff.DKGParticipant)
	for _, part := range data.Participants {
		participantLookup[part.ID] = &hotstuff.DKGParticipant{
			Id:             part.ID,
			PublicKeyShare: part.PubKeyShare.PublicKey,
			DKGIndex:       part.GroupIndex,
		}
	}

	return &hotstuff.DKGPublicData{
		GroupPubKey:           data.GroupPubKey.PublicKey,
		IdToDKGParticipantMap: participantLookup,
	}
}
