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

// AccountBasedMigration takes all the Payloads that belong to the given account
// and return the migrated Payloads
type AccountBasedMigration interface {
	InitMigration(
		log zerolog.Logger,
		allPayloads []*ledger.Payload,
		nWorker int,
	) error
	MigrateAccount(
		ctx context.Context,
		address common.Address,
		payloads []*ledger.Payload,
	) ([]*ledger.Payload, error)
}

func CreateAccountBasedMigration(
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

// MigrateByAccount teaks a migrator function and all the Payloads,
// and return the migrated Payloads
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
			log.With().Int("migration_index", i).Logger(),
			allPayloads,
			nWorker,
		); err != nil {
			return nil, fmt.Errorf("could not init migrator: %w", err)
		}
	}

	log.Info().
		Int("inner_migrations", len(migrations)).
		Int("nWorker", nWorker).
		Msgf("created account migrations")

	defer func() {
		for _, migrator := range migrations {
			// close the migrator if it's a Closer
			if migrator, ok := migrator.(io.Closer); ok {
				if err := migrator.Close(); err != nil {
					log.Error().Err(err).Msg("error closing migrator")
				}
			}
		}
	}()

	accountGroups := util.GroupPayloadsByAccount(log, allPayloads, nWorker)

	// migrate the Payloads under accounts
	migrated, err := MigrateGroupConcurrently(log, migrations, accountGroups, nWorker)

	if err != nil {
		return nil, fmt.Errorf("could not migrate group: %w", err)
	}

	log.Info().
		Int("account_count", accountGroups.Len()).
		Int("payload_count", len(allPayloads)).
		Msgf("finished migrating Payloads")

	return migrated, nil
}

// MigrateGroupConcurrently migrate the Payloads in the given payloadsByAccount map which
// using the migrator
// It's similar to MigrateGroupSequentially, except it will migrate different groups concurrently
func MigrateGroupConcurrently(
	log zerolog.Logger,
	migrations []AccountBasedMigration,
	accountGroups *util.PayloadAccountGrouping,
	nWorker int,
) ([]*ledger.Payload, error) {

	const logTopNDurations = 20

	ctx := context.Background()
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	jobs := make(chan jobMigrateAccountGroup, accountGroups.Len())

	wg := sync.WaitGroup{}
	wg.Add(nWorker)
	resultCh := make(chan *migrationResult, accountGroups.Len())
	for i := 0; i < int(nWorker); i++ {
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

					var err error
					accountMigrated := job.Payloads
					for _, migrator := range migrations {
						accountMigrated, err = migrator.MigrateAccount(ctx, job.Address, accountMigrated)
						if err != nil {
							break
						}
					}

					resultCh <- &migrationResult{
						migrationDuration: migrationDuration{
							Address:  job.Address,
							Duration: time.Since(start),
						},
						Migrated: accountMigrated,
						Err:      err,
					}
				}
			}
		}()
	}

	go func() {
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
	logAccount := moduleUtil.LogProgress("processing account group", accountGroups.Len(), log)

	migrated := make([]*ledger.Payload, 0)

	durations := &migrationDurations{}
	for i := 0; i < accountGroups.Len(); i++ {
		select {
		case <-ctx.Done():
			break
		default:
		}

		result := <-resultCh
		if result.Err != nil {
			cancel(result.Err)
			log.Error().
				Err(result.Err).
				Msg("error migrating account")
			break
		}

		if durations.Len() < logTopNDurations || result.Duration > (*durations)[0].Duration {
			if durations.Len() == logTopNDurations {
				heap.Pop(durations) // remove the element with the smallest duration
			}
			heap.Push(durations, result.migrationDuration)
		}

		accountMigrated := result.Migrated
		migrated = append(migrated, accountMigrated...)
		logAccount(1)
	}
	close(jobs)

	// make sure to exit all workers before returning from this function
	// so that the migrator can be closed properly
	wg.Wait()

	log.Info().
		Array("top_longest_migrations", durations.Array()).
		Msgf("Top longest migrations")

	if ctx.Err() != nil {
		return nil, fmt.Errorf("fail to migrate payload: %w", ctx.Err())
	}

	return migrated, nil
}

type jobMigrateAccountGroup struct {
	Address  common.Address
	Payloads []*ledger.Payload
}

type migrationResult struct {
	migrationDuration

	Migrated []*ledger.Payload
	Err      error
}

type migrationDuration struct {
	Address  common.Address
	Duration time.Duration
}

// implement heap methods for the timer results
type migrationDurations []migrationDuration

func (h *migrationDurations) Len() int { return len(*h) }
func (h *migrationDurations) Less(i, j int) bool {
	return (*h)[i].Duration < (*h)[j].Duration
}
func (h *migrationDurations) Swap(i, j int) {
	(*h)[i], (*h)[j] = (*h)[j], (*h)[i]
}
func (h *migrationDurations) Push(x interface{}) {
	*h = append(*h, x.(migrationDuration))
}
func (h *migrationDurations) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h *migrationDurations) Array() zerolog.LogArrayMarshaler {
	array := zerolog.Arr()
	for _, result := range *h {
		array = array.Str(fmt.Sprintf("%s: %s", result.Address.Hex(), result.Duration.String()))
	}
	return array
}
