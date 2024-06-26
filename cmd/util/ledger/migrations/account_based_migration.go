package migrations

import (
	"container/heap"
	"context"
	"fmt"
	"io"
	syncAtomic "sync/atomic"
	"time"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
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

// NewAccountBasedMigration creates a migration function that migrates the payloads
// account by account using the given migrations
// accounts are processed concurrently using the given number of workers
// but each account is processed sequentially by the given migrations in order.
// The migrations InitMigration function is called once before the migration starts
// And the Close function is called once after the migration finishes if the migration
// is a finisher.
func NewAccountBasedMigration(
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

	for migrationIndex, migration := range migrations {
		logger := log.With().
			Int("migration_index", migrationIndex).
			Logger()

		err := migration.InitMigration(
			logger,
			registersByAccount,
			nWorker,
		)
		if err != nil {
			return fmt.Errorf("could not init migration: %w", err)
		}
	}

	err := withMigrations(log, migrations, func() error {
		return MigrateAccountsConcurrently(
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

// MigrateAccountsConcurrently migrate the registers in the given account groups.
// The registers in each account are processed sequentially by the given migrations in order.
func MigrateAccountsConcurrently(
	log zerolog.Logger,
	migrations []AccountBasedMigration,
	registersByAccount *registers.ByAccount,
	nWorker int,
) error {

	accountCount := registersByAccount.AccountCount()

	g, ctx := errgroup.WithContext(context.Background())

	jobs := make(chan migrateAccountGroupJob, accountCount)
	results := make(chan migrationDuration, accountCount)

	workersLeft := int64(nWorker)

	for workerIndex := 0; workerIndex < nWorker; workerIndex++ {
		g.Go(func() error {
			defer func() {
				if syncAtomic.AddInt64(&workersLeft, -1) == 0 {
					close(results)
				}
			}()

			for job := range jobs {
				start := time.Now()

				address := job.Address
				accountRegisters := job.AccountRegisters

				// Only migrate accounts, not global registers
				if !util.IsServiceLevelAddress(address) {

					for migrationIndex, migration := range migrations {

						err := migration.MigrateAccount(ctx, address, accountRegisters)
						if err != nil {
							log.Err(err).
								Int("migration_index", migrationIndex).
								Type("migration", migration).
								Hex("address", address[:]).
								Msg("could not migrate account")
							return err
						}
					}
				}

				migrationDuration := migrationDuration{
					Address:       address,
					Duration:      time.Since(start),
					RegisterCount: accountRegisters.Count(),
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case results <- migrationDuration:
				}
			}

			return nil
		})
	}

	g.Go(func() error {
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
			return fmt.Errorf("failed to get all account registers: %w", err)
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
				return fmt.Errorf("failed to convert owner to address: %w", err)
			}

			job := migrateAccountGroupJob{
				Address:          address,
				AccountRegisters: accountRegisters,
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case jobs <- job:
			}
		}

		return nil
	})

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
			return duration.Duration < duration2.Duration
		},
	)

	g.Go(func() error {
		for duration := range results {
			topDurations.Add(duration)
			logAccount(1)
		}

		return nil
	})

	// make sure to exit all workers before returning from this function
	// so that the migration can be closed properly
	log.Info().Msg("waiting for migration workers to finish")
	err := g.Wait()
	if err != nil {
		return fmt.Errorf("failed to migrate accounts: %w", err)
	}

	log.Info().
		Array("top_longest_migrations", loggableMigrationDurations(topDurations)).
		Msgf("Top longest migrations")

	return nil
}

type migrateAccountGroupJob struct {
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
