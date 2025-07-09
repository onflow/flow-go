package retriever

import (
	"bytes"
	"context"
	"errors"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

type ComparingAccountRetriever struct {
	FailoverAccountRetriever
}

var _ AccountRetriever = (*ComparingAccountRetriever)(nil)

func NewComparingAccountRetriever(
	log zerolog.Logger,
	state protocol.State,
	localRequester AccountRetriever,
	execNodeRequester AccountRetriever,
) *ComparingAccountRetriever {
	return &ComparingAccountRetriever{
		FailoverAccountRetriever: FailoverAccountRetriever{
			log:               log.With().Str("account_retriever", "comparing").Logger(),
			state:             state,
			localRequester:    localRequester,
			execNodeRequester: execNodeRequester,
		},
	}
}

func (c *ComparingAccountRetriever) GetAccountAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
) (*flow.Account, error) {
	localAccount, localErr := c.localRequester.GetAccountAtBlock(ctx, address, blockID, height)
	if localErr == nil {
		return localAccount, nil
	}

	ENAccount, ENErr := c.execNodeRequester.GetAccountAtBlock(ctx, address, blockID, height)
	c.compareAccountResults(ENAccount, ENErr, localAccount, localErr, blockID, address)

	return ENAccount, ENErr
}

// compareAccountResults compares the result and error returned from local and remote getAccount calls
// and logs the results if they are different
func (c *ComparingAccountRetriever) compareAccountResults(
	execNodeResult *flow.Account,
	execErr error,
	localResult *flow.Account,
	localErr error,
	blockID flow.Identifier,
	address flow.Address,
) {
	if c.log.GetLevel() > zerolog.DebugLevel {
		return
	}

	lgCtx := c.log.With().
		Hex("block_id", blockID[:]).
		Str("address", address.String())

	// errors are different
	if !errors.Is(execErr, localErr) {
		lgCtx = lgCtx.
			AnErr("execution_node_error", execErr).
			AnErr("local_error", localErr)

		lg := lgCtx.Logger()
		lg.Debug().Msg("errors from getting account on local and EN do not match")
		return
	}

	// both errors are nil, compare the accounts
	if execErr == nil {
		lgCtx, ok := compareAccountsLogger(execNodeResult, localResult, lgCtx)
		if !ok {
			lg := lgCtx.Logger()
			lg.Debug().Msg("accounts from local and EN do not match")
		}
	}
}

// compareAccountsLogger compares accounts produced by the execution node and local storage and
// return a logger configured to log the differences
func compareAccountsLogger(exec, local *flow.Account, lgCtx zerolog.Context) (zerolog.Context, bool) {
	different := false

	if exec.Address != local.Address {
		lgCtx = lgCtx.
			Str("exec_node_address", exec.Address.String()).
			Str("local_address", local.Address.String())
		different = true
	}

	if exec.Balance != local.Balance {
		lgCtx = lgCtx.
			Uint64("exec_node_balance", exec.Balance).
			Uint64("local_balance", local.Balance)
		different = true
	}

	contractListMatches := true
	if len(exec.Contracts) != len(local.Contracts) {
		lgCtx = lgCtx.
			Int("exec_node_contract_count", len(exec.Contracts)).
			Int("local_contract_count", len(local.Contracts))
		contractListMatches = false
		different = true
	}

	missingContracts := zerolog.Arr()
	mismatchContracts := zerolog.Arr()

	for name, execContract := range exec.Contracts {
		localContract, ok := local.Contracts[name]

		if !ok {
			missingContracts.Str(name)
			contractListMatches = false
			different = true
		}

		if !bytes.Equal(execContract, localContract) {
			mismatchContracts.Str(name)
			different = true
		}
	}

	lgCtx = lgCtx.
		Array("missing_contracts", missingContracts).
		Array("mismatch_contracts", mismatchContracts)

	// only check if there were any missing
	if !contractListMatches {
		extraContracts := zerolog.Arr()
		for name := range local.Contracts {
			if _, ok := exec.Contracts[name]; !ok {
				extraContracts.Str(name)
				different = true
			}
		}
		lgCtx = lgCtx.Array("extra_contracts", extraContracts)
	}

	if len(exec.Keys) != len(local.Keys) {
		lgCtx = lgCtx.
			Int("exec_node_key_count", len(exec.Keys)).
			Int("local_key_count", len(local.Keys))
		different = true
	}

	mismatchKeys := zerolog.Arr()

	for i, execKey := range exec.Keys {
		localKey := local.Keys[i]

		if !execKey.PublicKey.Equals(localKey.PublicKey) {
			mismatchKeys.Uint32(execKey.Index)
			different = true
		}
	}

	lgCtx = lgCtx.Array("mismatch_keys", mismatchKeys)

	return lgCtx, !different
}
