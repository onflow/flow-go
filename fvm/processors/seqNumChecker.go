package processors

import (
	"fmt"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"

	"github.com/onflow/flow-go/fvm/context"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

type SequenceNumberChecker struct{}

func NewSequenceNumberChecker() *SequenceNumberChecker {
	return &SequenceNumberChecker{}
}

func (SequenceNumberChecker) CheckAndIncrement(
	ctx *context.Context,
	tx *flow.TransactionBody,
	sth *state.StateHolder,
	programs *programs.Programs,
	parentSpan opentracing.Span,
) error {
	if parentSpan != nil {
		span := ctx.Tracer.StartSpanFromParent(parentSpan, trace.FVMSeqNumCheckTransaction)
		span.LogFields(
			log.String("transaction.ID", tx.ID().String()),
		)
		defer span.Finish()
	}

	parentState := sth.State()
	childState := sth.NewChild()
	defer func() {
		if mergeError := parentState.MergeState(childState); mergeError != nil {
			panic(mergeError)
		}
		sth.SetActiveState(parentState)
	}()

	accounts := state.NewAccounts(sth)
	proposalKey := tx.ProposalKey

	accountKey, err := accounts.GetPublicKey(proposalKey.Address, proposalKey.KeyIndex)
	if err != nil {
		err = errors.NewInvalidProposalSignatureError(proposalKey.Address, proposalKey.KeyIndex, err)
		return fmt.Errorf("checking sequence number failed: %w", err)
	}

	if accountKey.Revoked {
		err = fmt.Errorf("proposal key has been revoked")
		err = errors.NewInvalidProposalSignatureError(proposalKey.Address, proposalKey.KeyIndex, err)
		return fmt.Errorf("checking sequence number failed: %w", err)
	}

	// Note that proposal key verification happens at the txVerifier and not here.

	valid := accountKey.SeqNumber == proposalKey.SequenceNumber

	if !valid {
		return errors.NewInvalidProposalSeqNumberError(proposalKey.Address, proposalKey.KeyIndex, accountKey.SeqNumber, proposalKey.SequenceNumber)
	}

	accountKey.SeqNumber++

	_, err = accounts.SetPublicKey(proposalKey.Address, proposalKey.KeyIndex, accountKey)
	if err != nil {
		childState.View().DropDelta()
		return fmt.Errorf("checking sequence number failed: %w", err)
	}
	return nil
}
