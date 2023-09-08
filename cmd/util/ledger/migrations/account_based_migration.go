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
	"github.com/onflow/flow-go/model/flow"
	moduleUtil "github.com/onflow/flow-go/module/util"
)

// AccountMigrator takes all the Payloads that belong to the given account
// and return the migrated Payloads
type AccountMigrator interface {
	MigratePayloads(ctx context.Context, address common.Address, payloads []*ledger.Payload) ([]*ledger.Payload, error)
}

// AccountMigratorFactory creates an AccountMigrator
type AccountMigratorFactory func(allPayloads []ledger.Payload, nWorker int) (AccountMigrator, error)

func CreateAccountBasedMigration(
	log zerolog.Logger,
	migratorFactory AccountMigratorFactory,
	nWorker int,
) func(payloads []ledger.Payload) ([]ledger.Payload, error) {
	return func(payloads []ledger.Payload) ([]ledger.Payload, error) {
		return MigrateByAccount(
			log,
			migratorFactory,
			payloads,
			nWorker,
		)
	}
}

// totalExpectedAccounts is just used to preallocate the account group  map
const totalExpectedAccounts = 40_000_000

// MigrateByAccount teaks a migrator function and all the Payloads, and return the migrated Payloads
func MigrateByAccount(
	log zerolog.Logger,
	migratorFactory AccountMigratorFactory,
	allPayloads []ledger.Payload,
	nWorker int,
) (
	[]ledger.Payload,
	error,
) {
	groups := &payloadGroup{
		NonAccountPayloads: make([]*ledger.Payload, 0),
		Accounts:           make(map[common.Address][]*ledger.Payload, totalExpectedAccounts),
	}

	log.Info().Msgf("start grouping for a total of %v Payloads", len(allPayloads))

	var err error
	logGrouping := moduleUtil.LogProgress("grouping payload", len(allPayloads), log)
	for i, payload := range allPayloads {
		groups, err = payloadGrouping(groups, payload)
		if err != nil {
			return nil, err
		}
		logGrouping(i)
	}

	log.Info().Msgf("finish grouping for Payloads by account: %v groups in total, %v NonAccountPayloads",
		len(groups.Accounts), len(groups.NonAccountPayloads))

	migrator, err := migratorFactory(allPayloads, nWorker)
	if err != nil {
		return nil, fmt.Errorf("could not create account migrator: %w", err)
	}

	log.Info().
		Str("migrator", fmt.Sprintf("%T", migrator)).
		Int("nWorker", nWorker).
		Msgf("created migrator")

	defer func() {
		// close the migrator if it's a Closer
		if migrator, ok := migrator.(io.Closer); ok {
			if err := migrator.Close(); err != nil {
				log.Error().Err(err).Msg("error closing migrator")
			}
		}
	}()

	// migrate the Payloads under accounts
	migrated, err := MigrateGroupConcurrently(log, migrator, groups.Accounts, nWorker)

	if err != nil {
		return nil, fmt.Errorf("could not migrate group: %w", err)
	}

	log.Info().Msgf("finished migrating Payloads for %v account", len(groups.Accounts))

	// add the non accounts which don't need to be migrated
	migrated = append(migrated, groups.NonAccountPayloads...)

	final := make([]ledger.Payload, 0, len(migrated))
	for _, p := range migrated {
		final = append(final, *p)
	}

	log.Info().Msgf("finished migrating all account based Payloads, total migrated Payloads: %v", len(migrated))

	return final, nil
}

// MigrateGroupConcurrently migrate the Payloads in the given payloadsByAccount map which
// using the migrator
// It's similar to MigrateGroupSequentially, except it will migrate different groups concurrently
func MigrateGroupConcurrently(
	log zerolog.Logger,
	migrator AccountMigrator,
	payloadsByAccount map[common.Address][]*ledger.Payload,
	nWorker int,
) ([]*ledger.Payload, error) {

	const logTopNDurations = 20

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan jobMigrateAccountGroup, len(payloadsByAccount))

	wg := sync.WaitGroup{}
	wg.Add(nWorker)
	resultCh := make(chan *migrationResult, len(payloadsByAccount))
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

					accountMigrated, err := migrator.MigratePayloads(ctx, job.Address, job.Payloads)

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
		for address, payloads := range payloadsByAccount {
			select {
			case <-ctx.Done():
				return
			case jobs <- jobMigrateAccountGroup{
				Address:  address,
				Payloads: payloads,
			}:
			}
		}
	}()

	// read job results
	logAccount := moduleUtil.LogProgress("processing account group", len(payloadsByAccount), log)

	migrated := make([]*ledger.Payload, 0)

	durations := &migrationDurations{}
	var err error
	for i := 0; i < len(payloadsByAccount); i++ {
		result := <-resultCh
		err = result.Err
		if err != nil {
			cancel()
			log.Error().
				Err(err).
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
		logAccount(i)
	}
	close(jobs)

	// make sure to exit all workers before returning from this function
	// so that the migrator can be closed properly
	wg.Wait()

	log.Info().
		Array("top_longest_migrations", durations.Array()).
		Msgf("Top longest migrations")

	if err != nil {
		return nil, fmt.Errorf("fail to migrate payload: %w", err)
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

// payloadToAccount takes a payload and return:
// - (address, true, nil) if the payload is for an account, the account address is returned
// - ("", false, nil) if the payload is not for an account
// - ("", false, err) if running into any exception
func payloadToAccount(p *ledger.Payload) (common.Address, bool, error) {
	k, err := p.Key()
	if err != nil {
		return common.Address{}, false, fmt.Errorf("could not find key for payload: %w", err)
	}

	id, err := util.KeyToRegisterID(k)
	if err != nil {
		return common.Address{}, false, fmt.Errorf("error converting key to register ID")
	}

	if len([]byte(id.Owner)) != flow.AddressLength {
		return common.Address{}, false, nil
	}

	address, err := common.BytesToAddress([]byte(id.Owner))
	if err != nil {
		return common.Address{}, false, fmt.Errorf("invalid account address: %w", err)
	}

	// The zero address is used for global Payloads and is not an account
	if address == common.ZeroAddress {
		return address, false, nil
	}

	return address, true, nil
}

// payloadGroup groups Payloads by account.
// For global Payloads, it's stored under NonAccountPayloads field
type payloadGroup struct {
	NonAccountPayloads []*ledger.Payload
	Accounts           map[common.Address][]*ledger.Payload
}

// payloadGrouping is a reducer function that adds the given payload to the corresponding
// group under its account
func payloadGrouping(groups *payloadGroup, payload ledger.Payload) (*payloadGroup, error) {
	address, isAccount, err := payloadToAccount(&payload)
	if err != nil {
		return nil, err
	}

	if isAccount {
		_, exist := groups.Accounts[address]
		if !exist {
			// preallocate the slice to avoid reallocation
			// most accounts should have the 4 domains + a few special registers
			groups.Accounts[address] = make([]*ledger.Payload, 0, 10)
		}
		groups.Accounts[address] = append(groups.Accounts[address], &payload)
	} else {
		groups.NonAccountPayloads = append(groups.NonAccountPayloads, &payload)
	}

	return groups, nil
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
