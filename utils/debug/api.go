package debug

import (
	"context"
	"fmt"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/onflow/flow-go/access/legacy/convert"
	rpcConvert "github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
)

func GetExecutionAPIBlockHeader(
	client execution.ExecutionAPIClient,
	ctx context.Context,
	blockID flow.Identifier,
) (
	*flow.Header,
	error,
) {
	req := &execution.GetBlockHeaderByIDRequest{
		Id: blockID[:],
	}

	resp, err := client.GetBlockHeaderByID(ctx, req)
	if err != nil {
		return nil, err
	}

	blockHeader, err := rpcConvert.MessageToBlockHeader(resp.Block)
	if err != nil {
		return nil, err
	}
	untrusted := flow.UntrustedHeaderBody{
		ChainID:            blockHeader.ChainID,
		ParentID:           blockHeader.ParentID,
		Height:             blockHeader.Height,
		Timestamp:          blockHeader.Timestamp,
		ParentVoterIndices: blockHeader.ParentVoterIndices,
		ParentVoterSigData: blockHeader.ParentVoterSigData,
		ProposerID:         blockHeader.ProposerID,
		LastViewTC:         blockHeader.LastViewTC,
	}
	if !blockHeader.ContainsParentQC() {
		rootHeaderBody, err := flow.NewRootHeaderBody(untrusted)
		if err != nil {
			return nil, fmt.Errorf("could not build root header body: %w", err)
		}
		rootHeader, err := flow.NewHeader(flow.UntrustedHeader{
			HeaderBody:  *rootHeaderBody,
			PayloadHash: convert.MessageToIdentifier(resp.Block.PayloadHash),
		})
		if err != nil {
			return nil, fmt.Errorf("could not build root header: %w", err)
		}

		return rootHeader, nil
	}

	headerBody, err := flow.NewHeaderBody(untrusted)
	if err != nil {
		return nil, fmt.Errorf("could not build header body: %w", err)
	}
	header, err := flow.NewHeader(flow.UntrustedHeader{
		HeaderBody:  *headerBody,
		PayloadHash: blockHeader.PayloadHash,
	})
	if err != nil {
		return nil, fmt.Errorf("could not build header: %w", err)
	}

	return header, nil
}

func GetAccessAPIBlockHeader(
	client access.AccessAPIClient,
	ctx context.Context,
	blockID flow.Identifier,
) (
	*flow.Header,
	error,
) {
	req := &access.GetBlockHeaderByIDRequest{
		Id: blockID[:],
	}

	resp, err := client.GetBlockHeaderByID(ctx, req)
	if err != nil {
		return nil, err
	}

	blockHeader, err := rpcConvert.MessageToBlockHeader(resp.Block)
	if err != nil {
		return nil, err
	}
	untrusted := flow.UntrustedHeaderBody{
		ChainID:            blockHeader.ChainID,
		ParentID:           blockHeader.ParentID,
		Height:             blockHeader.Height,
		Timestamp:          blockHeader.Timestamp,
		ParentVoterIndices: blockHeader.ParentVoterIndices,
		ParentVoterSigData: blockHeader.ParentVoterSigData,
		ProposerID:         blockHeader.ProposerID,
		LastViewTC:         blockHeader.LastViewTC,
	}
	if !blockHeader.ContainsParentQC() {
		rootHeaderBody, err := flow.NewRootHeaderBody(untrusted)
		if err != nil {
			return nil, fmt.Errorf("could not build root header body: %w", err)
		}
		rootHeader, err := flow.NewHeader(flow.UntrustedHeader{
			HeaderBody:  *rootHeaderBody,
			PayloadHash: convert.MessageToIdentifier(resp.Block.PayloadHash),
		})
		if err != nil {
			return nil, fmt.Errorf("could not build root header: %w", err)
		}

		return rootHeader, nil
	}

	headerBody, err := flow.NewHeaderBody(untrusted)
	if err != nil {
		return nil, fmt.Errorf("could not build header body: %w", err)
	}
	header, err := flow.NewHeader(flow.UntrustedHeader{
		HeaderBody:  *headerBody,
		PayloadHash: blockHeader.PayloadHash,
	})
	if err != nil {
		return nil, fmt.Errorf("could not build header: %w", err)
	}

	return header, nil
}
