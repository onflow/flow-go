package migrations

import (
	"container/heap"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
	moduleUtil "github.com/onflow/flow-go/module/util"
)

// logTopNDurations is the number of longest migrations to log at the end of the migration
const logTopNDurations = 20

// AccountBasedMigration is an interface for migrations that migrate account by account
// concurrently getting all the payloads for each account at a time.
type AccountBasedMigration interface {
	InitMigration(
		log zerolog.Logger,
		allPayloads []*ledger.Payload,
		nWorkers int,
	) error
	MigrateAccount(
		ctx context.Context,
		address common.Address,
		payloads []*ledger.Payload,
	) ([]*ledger.Payload, error)
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
) func(payloads []*ledger.Payload) ([]*ledger.Payload, error) {
	return func(payloads []*ledger.Payload) ([]*ledger.Payload, error) {
		return MigrateByAccount(
			log,
			nWorker,
			payloads,
			migrations,
		)
	}
}

// MigrateByAccount takes migrations and all the Payloads,
// and returns the migrated Payloads.
func MigrateByAccount(
	log zerolog.Logger,
	nWorker int,
	allPayloads []*ledger.Payload,
	migrations []AccountBasedMigration,
) (
	[]*ledger.Payload,
	error,
) {
	if len(allPayloads) == 0 {
		return allPayloads, nil
	}

	for i, migrator := range migrations {
		if err := migrator.InitMigration(
			log.With().
				Int("migration_index", i).
				Logger(),
			allPayloads,
			nWorker,
		); err != nil {
			return nil, fmt.Errorf("could not init migration: %w", err)
		}
	}

	log.Info().
		Int("inner_migrations", len(migrations)).
		Int("nWorker", nWorker).
		Msgf("created account migrations")

	defer func() {
		for i, migrator := range migrations {
			log.Info().
				Int("migration_index", i).
				Type("migration", migrator).
				Msg("closing migration")
			if err := migrator.Close(); err != nil {
				log.Error().Err(err).Msg("error closing migration")
			}
		}
	}()

	// group the Payloads by account
	accountGroups := util.GroupPayloadsByAccount(log, allPayloads, nWorker)

	// migrate the Payloads under accounts
	migrated, err := MigrateGroupConcurrently(log, migrations, accountGroups, nWorker)

	if err != nil {
		return nil, fmt.Errorf("could not migrate accounts: %w", err)
	}

	log.Info().
		Int("account_count", accountGroups.Len()).
		Int("payload_count", len(allPayloads)).
		Msgf("finished migrating Payloads")

	return migrated, nil
}

// MigrateGroupConcurrently migrate the Payloads in the given account groups.
// It uses nWorker to process the Payloads concurrently. The Payloads in each account
// are processed sequentially by the given migrations in order.
func MigrateGroupConcurrently(
	log zerolog.Logger,
	migrations []AccountBasedMigration,
	accountGroups *util.PayloadAccountGrouping,
	nWorker int,
) ([]*ledger.Payload, error) {

	ctx := context.Background()
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	jobs := make(chan jobMigrateAccountGroup, accountGroups.Len())

	wg := sync.WaitGroup{}
	wg.Add(nWorker)
	resultCh := make(chan *migrationResult, accountGroups.Len())
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

					// This is not an account, but service level keys.
					if util.IsServiceLevelAddress(job.Address) {
						resultCh <- &migrationResult{
							migrationDuration: migrationDuration{
								Address:      job.Address,
								Duration:     time.Since(start),
								PayloadCount: len(job.Payloads),
							},
							Migrated: job.Payloads,
						}
						continue
					}

					if _, ok := knownProblematicAccounts[job.Address]; ok {
						log.Info().
							Hex("address", job.Address[:]).
							Int("payload_count", len(job.Payloads)).
							Msg("skipping problematic account")
						resultCh <- &migrationResult{
							migrationDuration: migrationDuration{
								Address:      job.Address,
								Duration:     time.Since(start),
								PayloadCount: len(job.Payloads),
							},
							Migrated: job.Payloads,
						}
						continue
					}

					var err error
					accountMigrated := job.Payloads
					for m, migrator := range migrations {

						select {
						case <-ctx.Done():
							return
						default:
						}

						accountMigrated, err = migrator.MigrateAccount(ctx, job.Address, accountMigrated)
						if err != nil {
							log.Error().
								Err(err).
								Int("migration_index", m).
								Type("migration", migrator).
								Hex("address", job.Address[:]).
								Msg("could not migrate account")
							cancel(fmt.Errorf("could not migrate account: %w", err))
							return
						}
					}

					resultCh <- &migrationResult{
						migrationDuration: migrationDuration{
							Address:      job.Address,
							Duration:     time.Since(start),
							PayloadCount: len(job.Payloads),
						},
						Migrated: accountMigrated,
					}
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for {
			g, err := accountGroups.Next()
			if err != nil {
				cancel(fmt.Errorf("could not get next account group: %w", err))
				return
			}

			if g == nil {
				break
			}

			job := jobMigrateAccountGroup{
				Address:  g.Address,
				Payloads: g.Payloads,
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
			accountGroups.Len(),
		),
	)

	migrated := make([]*ledger.Payload, 0, accountGroups.AllPayloadsCount())
	durations := newMigrationDurations(logTopNDurations)
	contextDone := false
	for i := 0; i < accountGroups.Len(); i++ {
		select {
		case <-ctx.Done():
			contextDone = true
			break
		case result := <-resultCh:
			durations.Add(result)

			accountMigrated := result.Migrated
			migrated = append(migrated, accountMigrated...)
			logAccount(1)
		}
		if contextDone {
			break
		}
	}

	// make sure to exit all workers before returning from this function
	// so that the migrator can be closed properly
	log.Info().Msg("waiting for migration workers to finish")
	wg.Wait()

	log.Info().
		Array("top_longest_migrations", durations.Array()).
		Msgf("Top longest migrations")

	err := ctx.Err()
	if err != nil {
		return nil, fmt.Errorf("failed to migrate payload: %w", err)
	}

	return migrated, nil
}

var knownProblematicAccounts = map[common.Address]string{
	// Testnet accounts with broken contracts
	mustHexToAddress("434a1f199a7ae3ba"): "Broken contract FanTopPermission",
	mustHexToAddress("454c9991c2b8d947"): "Broken contract Test",
	mustHexToAddress("48602d8056ff9d93"): "Broken contract FanTopPermission",
	mustHexToAddress("5d63c34d7f05e5a4"): "Broken contract FanTopPermission",
	mustHexToAddress("5e3448b3cffb97f2"): "Broken contract FanTopPermission",
	mustHexToAddress("7d8c7e050c694eaa"): "Broken contract Test",
	mustHexToAddress("ba53f16ede01972d"): "Broken contract FanTopPermission",
	mustHexToAddress("c843c1f5a4805c3a"): "Broken contract FanTopPermission",
	mustHexToAddress("48d3be92e6e4a973"): "Broken contract FanTopPermission",
	// Mainnet account
}

func mustHexToAddress(hex string) common.Address {
	address, err := common.HexToAddress(hex)
	if err != nil {
		panic(err)
	}
	return address
}

type jobMigrateAccountGroup struct {
	Address  common.Address
	Payloads []*ledger.Payload
}

type migrationResult struct {
	migrationDuration

	Migrated []*ledger.Payload
}

type migrationDuration struct {
	Address      common.Address
	Duration     time.Duration
	PayloadCount int
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
		array = array.Str(fmt.Sprintf("%s [payloads: %d]: %s",
			result.Address.Hex(),
			result.PayloadCount,
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
