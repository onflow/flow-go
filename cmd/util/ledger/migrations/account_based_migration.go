package migrations

import (
	"container/heap"
	"context"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"

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
	resultCh := make(chan *migrationResult, accountCount)
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
						resultCh <- &migrationResult{
							migrationDuration: migrationDuration{
								Address:       address,
								Duration:      time.Since(start),
								RegisterCount: accountRegisters.Count(),
							},
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

					resultCh <- &migrationResult{
						migrationDuration: migrationDuration{
							Address:       address,
							Duration:      time.Since(start),
							RegisterCount: accountRegisters.Count(),
						},
					}
				}
			}
		}()
	}

	go func() {
		defer close(jobs)

		allAccountRegisters := make([]*registers.AccountRegisters, 0, accountCount)

		err := registersByAccount.ForEachAccount(
			func(accountRegisters *registers.AccountRegisters) error {
				allAccountRegisters = append(
					allAccountRegisters,
					accountRegisters,
				)
				return nil
			},
		)
		if err != nil {
			cancel(fmt.Errorf("failed to get all account registers: %w", err))
		}

		// Schedule jobs in descending order of the number of registers
		sort.Slice(allAccountRegisters, func(i, j int) bool {
			a := allAccountRegisters[i]
			b := allAccountRegisters[j]
			return a.Count() > b.Count()
		})

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

	durations := newMigrationDurations(logTopNDurations)
accountLoop:
	for accountIndex := 0; accountIndex < accountCount; accountIndex++ {
		select {
		case <-ctx.Done():
			break accountLoop
		case result := <-resultCh:
			durations.Add(result)
			logAccount(1)
		}
	}

	// make sure to exit all workers before returning from this function
	// so that the migration can be closed properly
	log.Info().Msg("waiting for migration workers to finish")
	wg.Wait()

	log.Info().
		Array("top_longest_migrations", durations.Array()).
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

type jobMigrateAccountGroup struct {
	Address          common.Address
	AccountRegisters *registers.AccountRegisters
}

type migrationResult struct {
	migrationDuration
}

type migrationDuration struct {
	Address       common.Address
	Duration      time.Duration
	RegisterCount int
}

// migrationDurations implements heap methods for the timer results
type migrationDurations struct {
	v []migrationDuration

	KeepTopN int
}

// newMigrationDurations creates a new migrationDurations which are used to track the
// accounts that took the longest time to migrate.
func newMigrationDurations(keepTopN int) *migrationDurations {
	return &migrationDurations{
		v:        make([]migrationDuration, 0, keepTopN),
		KeepTopN: keepTopN,
	}
}

func (h *migrationDurations) Len() int { return len(h.v) }
func (h *migrationDurations) Less(i, j int) bool {
	return h.v[i].Duration < h.v[j].Duration
}
func (h *migrationDurations) Swap(i, j int) {
	h.v[i], h.v[j] = h.v[j], h.v[i]
}
func (h *migrationDurations) Push(x interface{}) {
	h.v = append(h.v, x.(migrationDuration))
}
func (h *migrationDurations) Pop() interface{} {
	old := h.v
	n := len(old)
	x := old[n-1]
	h.v = old[0 : n-1]
	return x
}

func (h *migrationDurations) Array() zerolog.LogArrayMarshaler {
	array := zerolog.Arr()
	for _, result := range h.v {
		array = array.Str(fmt.Sprintf("%s [registers: %d]: %s",
			result.Address.Hex(),
			result.RegisterCount,
			result.Duration.String(),
		))
	}
	return array
}

func (h *migrationDurations) Add(result *migrationResult) {
	if h.Len() < h.KeepTopN || result.Duration > h.v[0].Duration {
		if h.Len() == h.KeepTopN {
			heap.Pop(h) // remove the element with the smallest duration
		}
		heap.Push(h, result.migrationDuration)
	}
}
