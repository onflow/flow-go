package migrations

import (
	"bytes"
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
type AccountMigratorFactory func(allPayloads []*ledger.Payload, nWorker int) (AccountMigrator, error)

func CreateAccountBasedMigration(
	log zerolog.Logger,
	migratorFactory AccountMigratorFactory,
	nWorker int,
) func(payloads []*ledger.Payload) ([]*ledger.Payload, error) {
	return func(payloads []*ledger.Payload) ([]*ledger.Payload, error) {
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
	allPayloads []*ledger.Payload,
	nWorker int,
) (
	[]*ledger.Payload,
	error,
) {
	if len(allPayloads) == 0 {
		return allPayloads, nil
	}

	payloads := sortablePayloads(allPayloads)
	sort.Sort(payloads)

	i := 0
	accountIndexes := make([]int, 0, totalExpectedAccounts)
	accountIndexes = append(accountIndexes, i)
	for {
		j := payloads.FindLastOfTheSameKey(i)
		if j == len(payloads)-1 {
			break
		}
		accountIndexes = append(accountIndexes, j+1)
		i = j + 1
	}

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
	migrated, err := MigrateGroupConcurrently(log, migrator, payloads, accountIndexes, nWorker)

	if err != nil {
		return nil, fmt.Errorf("could not migrate group: %w", err)
	}

	log.Info().
		Int("account_count", len(accountIndexes)).
		Int("payload_count", len(payloads)).
		Msgf("finished migrating Payloads")

	return migrated, nil
}

// MigrateGroupConcurrently migrate the Payloads in the given payloadsByAccount map which
// using the migrator
// It's similar to MigrateGroupSequentially, except it will migrate different groups concurrently
func MigrateGroupConcurrently(
	log zerolog.Logger,
	migrator AccountMigrator,
	payloads sortablePayloads,
	accountIndexes []int,
	nWorker int,
) ([]*ledger.Payload, error) {

	const logTopNDurations = 20

	ctx := context.Background()
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	jobs := make(chan jobMigrateAccountGroup, len(accountIndexes))

	wg := sync.WaitGroup{}
	wg.Add(nWorker)
	resultCh := make(chan *migrationResult, len(accountIndexes))
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
		for i, accountStartIdex := range accountIndexes {
			accountEndIndex := 0
			if i == len(accountIndexes)-1 {
				accountEndIndex = len(payloads)
			} else {
				accountEndIndex = accountIndexes[i+1]
			}

			address, err := payloadToAddress(payloads[accountStartIdex])
			if err != nil {
				cancel(fmt.Errorf("could not get key for payload: %w", err))
				return
			}

			job := jobMigrateAccountGroup{
				Address:  address,
				Payloads: payloads[accountStartIdex:accountEndIndex],
			}

			select {
			case <-ctx.Done():
				return
			case jobs <- job:
			}
		}
	}()

	// read job results
	logAccount := moduleUtil.LogProgress("processing account group", len(accountIndexes), log)

	migrated := make([]*ledger.Payload, 0)

	durations := &migrationDurations{}
	for i := 0; i < len(accountIndexes); i++ {
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

// payloadToAddress takes a payload and return:
// - (address, true, nil) if the payload is for an account, the account address is returned
// - ("", false, nil) if the payload is not for an account
// - ("", false, err) if running into any exception
// The zero address is used for global Payloads and is not an account
func payloadToAddress(p *ledger.Payload) (common.Address, error) {
	k, err := p.Key()
	if err != nil {
		return common.ZeroAddress, fmt.Errorf("could not find key for payload: %w", err)
	}

	id, err := util.KeyToRegisterID(k)
	if err != nil {
		return common.ZeroAddress, fmt.Errorf("error converting key to register ID")
	}

	if len([]byte(id.Owner)) != flow.AddressLength {
		return common.ZeroAddress, nil
	}

	address, err := common.BytesToAddress([]byte(id.Owner))
	if err != nil {
		return common.ZeroAddress, fmt.Errorf("invalid account address: %w", err)
	}

	return address, nil
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

// EncodedKeyAddressPrefixLength is the length of the address prefix in the encoded key
// 2 for uint16 of number of key parts
// 4 for uint32 of the length of the first key part
// 8 for the address which is the actual length of the first key part
const EncodedKeyAddressPrefixLength = 2 + 4 + 8

type sortablePayloads []*ledger.Payload

func (s sortablePayloads) Len() int {
	return len(s)
}

func (s sortablePayloads) Less(i, j int) bool {
	return s.Compare(i, j) < 0
}

func (s sortablePayloads) Compare(i, j int) int {
	return bytes.Compare(
		s[i].EncodedKey()[:EncodedKeyAddressPrefixLength],
		s[j].EncodedKey()[:EncodedKeyAddressPrefixLength],
	)
}

func (s sortablePayloads) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s sortablePayloads) FindLastOfTheSameKey(i int) int {
	low := i
	step := 1
	for low+step < len(s) && s.Compare(low+step, i) == 0 {
		low += step
		step *= 2
	}

	high := low + step
	if high > len(s) {
		high = len(s)
	}

	for low < high {
		mid := (low + high) / 2
		if s.Compare(mid, i) == 0 {
			low = mid + 1
		} else {
			high = mid
		}
	}

	return low - 1
}
