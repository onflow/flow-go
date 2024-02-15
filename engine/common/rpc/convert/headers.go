package convert

import (
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/flow"
)

// BlockHeaderToMessage converts a flow.Header to a protobuf message
func BlockHeaderToMessage(
	h *flow.Header,
	signerIDs flow.IdentifierList,
) (*entities.BlockHeader, error) {
	id := h.ID()

	t := timestamppb.New(h.Timestamp)
	var lastViewTC *entities.TimeoutCertificate
	if h.LastViewTC != nil {
		newestQC := h.LastViewTC.NewestQC
		lastViewTC = &entities.TimeoutCertificate{
			View:          h.LastViewTC.View,
			HighQcViews:   h.LastViewTC.NewestQCViews,
			SignerIndices: h.LastViewTC.SignerIndices,
			SigData:       h.LastViewTC.SigData,
			HighestQc: &entities.QuorumCertificate{
				View:          newestQC.View,
				BlockId:       newestQC.BlockID[:],
				SignerIndices: newestQC.SignerIndices,
				SigData:       newestQC.SigData,
			},
		}
	}
	parentVoterIds := IdentifiersToMessages(signerIDs)

	return &entities.BlockHeader{
		Id:                 id[:],
		ParentId:           h.ParentID[:],
		Height:             h.Height,
		PayloadHash:        h.PayloadHash[:],
		Timestamp:          t,
		View:               h.View,
		ParentView:         h.ParentView,
		ParentVoterIndices: h.ParentVoterIndices,
		ParentVoterIds:     parentVoterIds,
		ParentVoterSigData: h.ParentVoterSigData,
		ProposerId:         h.ProposerID[:],
		ProposerSigData:    h.ProposerSigData,
		ChainId:            h.ChainID.String(),
		LastViewTc:         lastViewTC,
	}, nil
}

// MessageToBlockHeader converts a protobuf message to a flow.Header
func MessageToBlockHeader(m *entities.BlockHeader) (*flow.Header, error) {
	chainId, err := MessageToChainId(m.ChainId)
	if err != nil {
		return nil, fmt.Errorf("failed to convert ChainId: %w", err)
	}
	var lastViewTC *flow.TimeoutCertificate
	if m.LastViewTc != nil {
		newestQC := m.LastViewTc.HighestQc
		if newestQC == nil {
			return nil, fmt.Errorf("invalid structure newest QC should be present")
		}
		lastViewTC = &flow.TimeoutCertificate{
			View:          m.LastViewTc.View,
			NewestQCViews: m.LastViewTc.HighQcViews,
			SignerIndices: m.LastViewTc.SignerIndices,
			SigData:       m.LastViewTc.SigData,
			NewestQC: &flow.QuorumCertificate{
				View:          newestQC.View,
				BlockID:       MessageToIdentifier(newestQC.BlockId),
				SignerIndices: newestQC.SignerIndices,
				SigData:       newestQC.SigData,
			},
		}
	}

	return &flow.Header{
		ParentID:           MessageToIdentifier(m.ParentId),
		Height:             m.Height,
		PayloadHash:        MessageToIdentifier(m.PayloadHash),
		Timestamp:          m.Timestamp.AsTime(),
		View:               m.View,
		ParentView:         m.ParentView,
		ParentVoterIndices: m.ParentVoterIndices,
		ParentVoterSigData: m.ParentVoterSigData,
		ProposerID:         MessageToIdentifier(m.ProposerId),
		ProposerSigData:    m.ProposerSigData,
		ChainID:            *chainId,
		LastViewTC:         lastViewTC,
	}, nil
}
