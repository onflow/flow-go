package util

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
)

func NewByAccountRegistersFromPayloadAccountGrouping(
	payloadAccountGrouping *PayloadAccountGrouping,
	nWorker int,
) (
	*registers.ByAccount,
	error,
) {
	accountCount := payloadAccountGrouping.Len()

	if accountCount == 0 {
		return registers.NewByAccount(), nil
	}

	// Set nWorker to be the lesser of nWorker and accountCount
	// but greater than 0.
	nWorker = min(nWorker, accountCount)
	nWorker = max(nWorker, 1)

	g, ctx := errgroup.WithContext(context.Background())

	jobs := make(chan *PayloadAccountGroup, nWorker)
	results := make(chan *registers.AccountRegisters, nWorker)

	g.Go(func() error {
		defer close(jobs)
		for {
			payloadAccountGroup, err := payloadAccountGrouping.Next()
			if err != nil {
				return fmt.Errorf("failed to group payloads by account: %w", err)
			}

			if payloadAccountGroup == nil {
				return nil
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case jobs <- payloadAccountGroup:
			}
		}
	})

	workersLeft := int64(nWorker)
	for i := 0; i < nWorker; i++ {
		g.Go(func() error {
			defer func() {
				if atomic.AddInt64(&workersLeft, -1) == 0 {
					close(results)
				}
			}()

			for payloadAccountGroup := range jobs {

				// Convert address to owner
				payloadGroupOwner := flow.AddressToRegisterOwner(payloadAccountGroup.Address)

				accountRegisters, err := registers.NewAccountRegistersFromPayloads(
					payloadGroupOwner,
					payloadAccountGroup.Payloads,
				)
				if err != nil {
					return fmt.Errorf("failed to create account registers from payloads: %w", err)
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case results <- accountRegisters:
				}
			}

			return nil
		})
	}

	registersByAccount := registers.NewByAccount()
	g.Go(func() error {
		for accountRegisters := range results {
			oldAccountRegisters := registersByAccount.SetAccountRegisters(accountRegisters)
			if oldAccountRegisters != nil {
				// Account grouping should never create multiple groups for an account.
				// In case it does anyway, merge the groups together,
				// by merging the existing registers into the new ones.

				log.Warn().Msgf(
					"account registers already exist for account %x. merging %d existing registers into %d new",
					accountRegisters.Owner(),
					oldAccountRegisters.Count(),
					accountRegisters.Count(),
				)

				err := accountRegisters.Merge(oldAccountRegisters)
				if err != nil {
					return fmt.Errorf("failed to merge account registers: %w", err)
				}
			}
		}

		return nil
	})

	return registersByAccount, g.Wait()
}
