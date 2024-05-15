package migrations

import (
	"container/heap"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
	moduleUtil "github.com/onflow/flow-go/module/util"
)

// logTopNDurations is the number of longest migrations to log at the end of the migration
const logTopNDurations = 20

// AccountBasedMigration is an interface for migrations that migrate account by account
// concurrently getting all the payloads for each account at a time.
type AccountBasedMigration interface {
	InitMigration(
		log zerolog.Logger,
		registersByAccount *registers.ByAccount,
		nWorkers int,
	) error
	MigrateAccount(
		ctx context.Context,
		address common.Address,
		accountRegisters *registers.AccountRegisters,
	) error
	io.Closer
}

// CreateAccountBasedMigration creates a migration function that migrates the payloads
// account by account using the given migrations
// accounts are processed concurrently using the given number of workers
// but each account is processed sequentially by the given migrations in order.
// The migrations InitMigration function is called once before the migration starts
// And the Close function is called once after the migration finishes if the migration
// is a finisher.
func CreateAccountBasedMigration(
	log zerolog.Logger,
	nWorker int,
	migrations []AccountBasedMigration,
) RegistersMigration {
	return func(registersByAccount *registers.ByAccount) error {
		return MigrateByAccount(
			log,
			nWorker,
			registersByAccount,
			migrations,
		)
	}
}

// MigrateByAccount takes migrations and all the registers, grouped by account,
// and returns the migrated registers.
func MigrateByAccount(
	log zerolog.Logger,
	nWorker int,
	registersByAccount *registers.ByAccount,
	migrations []AccountBasedMigration,
) error {
	accountCount := registersByAccount.AccountCount()

	if accountCount == 0 {
		return nil
	}

	log.Info().
		Int("inner_migrations", len(migrations)).
		Int("nWorker", nWorker).
		Msgf("created account migrations")

	for i, migration := range migrations {
		err := migration.InitMigration(
			log.With().
				Int("migration_index", i).
				Logger(),
			registersByAccount,
			nWorker,
		)
		if err != nil {
			return fmt.Errorf("could not init migration: %w", err)
		}
	}

	err := withMigrations(log, migrations, func() error {
		return MigrateGroupConcurrently(
			log,
			migrations,
			registersByAccount,
			nWorker,
		)
	})

	log.Info().
		Int("account_count", accountCount).
		Msgf("finished migrating registers")

	if err != nil {
		return fmt.Errorf("could not migrate accounts: %w", err)
	}

	return nil
}

// withMigrations calls the given function and then closes the given migrations.
func withMigrations(
	log zerolog.Logger,
	migrations []AccountBasedMigration,
	f func() error,
) (err error) {
	defer func() {
		for migrationIndex, migration := range migrations {
			log.Info().
				Int("migration_index", migrationIndex).
				Type("migration", migration).
				Msg("closing migration")
			if cerr := migration.Close(); cerr != nil {
				log.Err(cerr).Msg("error closing migration")
				if err == nil {
					// only set the error if it's not already set
					// so that we don't overwrite the original error
					err = cerr
				}
			}
		}
	}()

	return f()
}

// MigrateGroupConcurrently migrate the registers in the given account groups.
// The registers in each account are processed sequentially by the given migrations in order.
func MigrateGroupConcurrently(
	log zerolog.Logger,
	migrations []AccountBasedMigration,
	registersByAccount *registers.ByAccount,
	nWorker int,
) error {

	ctx := context.Background()
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	accountCount := registersByAccount.AccountCount()

	jobs := make(chan jobMigrateAccountGroup, accountCount)

	wg := sync.WaitGroup{}
	wg.Add(nWorker)
	resultCh := make(chan migrationDuration, accountCount)
	for i := 0; i < nWorker; i++ {
		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobs:
					if !ok {
						return
					}
					start := time.Now()

					address := job.Address
					accountRegisters := job.AccountRegisters

					// This is not an account, but service level keys.
					if util.IsServiceLevelAddress(address) {
						resultCh <- migrationDuration{
							Address:       address,
							Duration:      time.Since(start),
							RegisterCount: accountRegisters.Count(),
						}
						continue
					}

					for m, migration := range migrations {

						select {
						case <-ctx.Done():
							return
						default:
						}

						err := migration.MigrateAccount(ctx, address, accountRegisters)
						if err != nil {
							log.Error().
								Err(err).
								Int("migration_index", m).
								Type("migration", migration).
								Hex("address", address[:]).
								Msg("could not migrate account")
							cancel(fmt.Errorf("could not migrate account: %w", err))
							return
						}
					}

					resultCh <- migrationDuration{
						Address:       address,
						Duration:      time.Since(start),
						RegisterCount: accountRegisters.Count(),
					}
				}
			}
		}()
	}

	go func() {
		defer close(jobs)

		// TODO: maybe adjust, make configurable, or dependent on chain
		const keepTopNAccountRegisters = 20
		largestAccountRegisters := util.NewTopN[*registers.AccountRegisters](
			keepTopNAccountRegisters,
			func(a, b *registers.AccountRegisters) bool {
				return a.Count() < b.Count()
			},
		)

		allAccountRegisters := make([]*registers.AccountRegisters, accountCount)

		smallerAccountRegisterIndex := keepTopNAccountRegisters
		err := registersByAccount.ForEachAccount(
			func(accountRegisters *registers.AccountRegisters) error {

				// Try to add the account registers to the top N largest account registers.
				// If there is an "overflow" element (either the added element, or an existing element),
				// add it to the account registers.
				// This way we can process the largest account registers first,
				// and do not need to sort all account registers.

				popped, didPop := largestAccountRegisters.Add(accountRegisters)
				if didPop {
					allAccountRegisters[smallerAccountRegisterIndex] = popped
					smallerAccountRegisterIndex++
				}

				return nil
			},
		)
		if err != nil {
			cancel(fmt.Errorf("failed to get all account registers: %w", err))
		}

		// Add the largest account registers to the account registers.
		// The elements in the top N largest account registers are returned in reverse order.
		for index := largestAccountRegisters.Len() - 1; index >= 0; index-- {
			accountRegisters := heap.Pop(largestAccountRegisters).(*registers.AccountRegisters)
			allAccountRegisters[index] = accountRegisters
		}

		for _, accountRegisters := range allAccountRegisters {
			owner := accountRegisters.Owner()

			address, err := common.BytesToAddress([]byte(owner))
			if err != nil {
				cancel(fmt.Errorf("failed to convert owner to address: %w", err))
				return
			}

			job := jobMigrateAccountGroup{
				Address:          address,
				AccountRegisters: accountRegisters,
			}

			select {
			case <-ctx.Done():
				return
			case jobs <- job:
			}
		}
	}()

	// read job results
	logAccount := moduleUtil.LogProgress(
		log,
		moduleUtil.DefaultLogProgressConfig(
			"processing account group",
			accountCount,
		),
	)

	topDurations := util.NewTopN[migrationDuration](
		logTopNDurations,
		func(duration migrationDuration, duration2 migrationDuration) bool {
			return duration.Duration > duration2.Duration
		},
	)

accountLoop:
	for accountIndex := 0; accountIndex < accountCount; accountIndex++ {
		select {
		case <-ctx.Done():
			break accountLoop
		case duration := <-resultCh:
			topDurations.Add(duration)
			logAccount(1)
		}
	}

	// make sure to exit all workers before returning from this function
	// so that the migration can be closed properly
	log.Info().Msg("waiting for migration workers to finish")
	wg.Wait()

	log.Info().
		Array("top_longest_migrations", loggableMigrationDurations(topDurations)).
		Msgf("Top longest migrations")

	err := ctx.Err()
	if err != nil {
		cause := context.Cause(ctx)
		if cause != nil {
			err = cause
		}

		return fmt.Errorf("failed to migrate payload: %w", err)
	}

	return nil
}

var TestnetAccountsWithBrokenSlabReferences = func() map[common.Address]struct{} {
	testnetAddresses := map[common.Address]struct{}{
		mustHexToAddress("434a1f199a7ae3ba"): {},
		mustHexToAddress("454c9991c2b8d947"): {},
		mustHexToAddress("48602d8056ff9d93"): {},
		mustHexToAddress("5d63c34d7f05e5a4"): {},
		mustHexToAddress("5e3448b3cffb97f2"): {},
		mustHexToAddress("7d8c7e050c694eaa"): {},
		mustHexToAddress("ba53f16ede01972d"): {},
		mustHexToAddress("c843c1f5a4805c3a"): {},
		mustHexToAddress("48d3be92e6e4a973"): {},
	}

	for address := range testnetAddresses {
		if !flow.Testnet.Chain().IsValid(flow.Address(address)) {
			panic(fmt.Sprintf("invalid testnet address: %s", address.Hex()))
		}
	}

	return testnetAddresses
}()

func mustHexToAddress(hex string) common.Address {
	address, err := common.HexToAddress(hex)
	if err != nil {
		panic(err)
	}
	return address
}

type jobMigrateAccountGroup struct {
	Address          common.Address
	AccountRegisters *registers.AccountRegisters
}

type migrationDuration struct {
	Address       common.Address
	Duration      time.Duration
	RegisterCount int
}

func loggableMigrationDurations(durations *util.TopN[migrationDuration]) zerolog.LogArrayMarshaler {
	array := zerolog.Arr()

	for index := durations.Len() - 1; index >= 0; index-- {
		duration := heap.Pop(durations).(migrationDuration)
		array = array.Str(fmt.Sprintf(
			"%s [registers: %d]: %s",
			duration.Address.Hex(),
			duration.RegisterCount,
			duration.Duration.String(),
		))
	}

	return array
}
