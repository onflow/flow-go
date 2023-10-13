package migrations

import (
	"context"
	"fmt"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	util2 "github.com/onflow/flow-go/module/util"
	"github.com/rs/zerolog"
	"sync"

	"github.com/onflow/cadence/runtime/interpreter"
)

var cricketMomentsAddress = mustHexToAddress("4eded0de73020ca5")
var cricketMomentsShardedCollectionType = "A.4eded0de73020ca5.CricketMomentsShardedCollection.ShardedCollection"

func isCricketMomentsShardedCollection(
	mr *migratorRuntime,
	value interpreter.Value,
) bool {

	if mr.Address != cricketMomentsAddress {
		return false
	}

	compositeValue, ok := value.(*interpreter.CompositeValue)
	if !ok {
		return false
	}

	return string(compositeValue.TypeID()) == cricketMomentsShardedCollectionType
}

func getCricketMomentsShardedCollectionNFTCount(
	mr *migratorRuntime,
	shardedCollectionMap *interpreter.DictionaryValue,
) (int, error) {
	// count all values so we can track progress
	count := 0
	shardedCollectionMapIterator := shardedCollectionMap.Iterator()
	for {
		key := shardedCollectionMapIterator.NextKey(nil)
		if key == nil {
			break
		}

		ownedNFTs, err := getNftCollection(mr.Interpreter, key, shardedCollectionMap)
		if err != nil {
			return 0, err
		}

		count += ownedNFTs.Count()
	}
	return count, nil
}

func getShardedCollectionMap(mr *migratorRuntime, value interpreter.Value) (*interpreter.DictionaryValue, error) {
	shardedCollectionResource, ok := value.(*interpreter.CompositeValue)
	if !ok {
		return nil, fmt.Errorf("expected *interpreter.CompositeValue, got %T", value)
	}
	shardedCollectionMapField := shardedCollectionResource.GetField(
		mr.Interpreter,
		interpreter.EmptyLocationRange,
		"collections",
	)
	if shardedCollectionMapField == nil {
		return nil, fmt.Errorf("expected collections field")
	}
	shardedCollectionMap, ok := shardedCollectionMapField.(*interpreter.DictionaryValue)
	if !ok {
		return nil, fmt.Errorf("expected collections to be *interpreter.DictionaryValue, got %T", shardedCollectionMapField)
	}
	return shardedCollectionMap, nil
}

func getNftCollection(
	inter *interpreter.Interpreter,
	outerKey interpreter.Value,
	shardedCollectionMap *interpreter.DictionaryValue,
) (*interpreter.DictionaryValue, error) {
	value := shardedCollectionMap.GetKey(
		inter,
		interpreter.EmptyLocationRange,
		outerKey,
	)

	if value == nil {
		return nil, fmt.Errorf("expected value for key %s", outerKey)
	}

	someCollection, ok := value.(*interpreter.SomeValue)
	if !ok {
		return nil, fmt.Errorf("expected collection to be *interpreter.SomeValue, got %T", value)
	}

	collection, ok := someCollection.InnerValue(
		inter,
		interpreter.EmptyLocationRange).(*interpreter.CompositeValue)
	if !ok {
		return nil, fmt.Errorf("expected inner collection to be *interpreter.CompositeValue, got %T", value)
	}

	ownedNFTsRaw := collection.GetField(
		inter,
		interpreter.EmptyLocationRange,
		"ownedNFTs",
	)
	if ownedNFTsRaw == nil {
		return nil, fmt.Errorf("expected ownedNFTs field")
	}
	ownedNFTs, ok := ownedNFTsRaw.(*interpreter.DictionaryValue)
	if !ok {
		return nil, fmt.Errorf("expected ownedNFTs to be *interpreter.DictionaryValue, got %T", ownedNFTsRaw)
	}

	return ownedNFTs, nil
}

type cricketKeyPair struct {
	shardedCollectionKey interpreter.Value
	nftCollectionKey     interpreter.Value
}

func cloneCricketMomentsShardedCollection(
	log zerolog.Logger,
	nWorkers int,
	mr *migratorRuntime,
	domain string,
	key interpreter.StorageMapKey,
	value interpreter.Value,
) (interpreter.Value, error) {

	shardedCollectionMap, err := getShardedCollectionMap(mr, value)
	if err != nil {
		return nil, err
	}
	count, err := getCricketMomentsShardedCollectionNFTCount(
		mr,
		shardedCollectionMap,
	)
	if err != nil {
		return nil, err
	}

	progressLog := util2.LogProgress(log, "cloning cricket moments", count)

	ctx, c := context.WithCancelCause(context.Background())
	cancel := func(err error) {
		if err != nil {
			log.Info().Err(err).Msg("canceling context")
		}
		c(err)
	}
	defer cancel(nil)

	type valueWithKeys struct {
		cricketKeyPair
		value interpreter.Value
	}

	keyPairChan := make(chan cricketKeyPair, count)
	clonedValues := make([]valueWithKeys, 0, count)
	wg := sync.WaitGroup{}
	wg.Add(nWorkers)

	// worker for dispatching values to clone
	go func() {
		defer close(keyPairChan)

		inter, _, err := mr.ChildInterpreter()
		if err != nil {
			cancel(err)
			return
		}

		storageMap := mr.GetReadOnlyStorage().GetStorageMap(mr.Address, domain, false)
		storageMapValue := storageMap.ReadValue(&util.NopMemoryGauge{}, key)
		scm, err := getShardedCollectionMap(mr, storageMapValue)
		if err != nil {
			cancel(err)
			return
		}

		shardedCollectionMapIterator := scm.Iterator()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			outerKey := shardedCollectionMapIterator.NextKey(nil)
			if outerKey == nil {
				break
			}

			ownedNFTs, err := getNftCollection(inter, outerKey, scm)
			if err != nil {
				cancel(err)
				return
			}

			ownedNFTsIterator := ownedNFTs.Iterator()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				innerKey := ownedNFTsIterator.NextKey(nil)
				if innerKey == nil {
					break
				}

				keyPairChan <- cricketKeyPair{
					nftCollectionKey:     innerKey,
					shardedCollectionKey: outerKey,
				}
			}
		}
	}()

	type interpreterWithClose struct {
		inter *interpreter.Interpreter
		close func() error
	}

	interpreters := make([]interpreterWithClose, 0, nWorkers)

	// workers for cloning values
	for i := 0; i < nWorkers; i++ {

		inter, c, err := mr.ChildInterpreter()
		if err != nil {
			return nil, err
		}
		interpreters = append(interpreters, interpreterWithClose{
			inter: inter,
			close: c,
		})
		go func(i int) {
			inter := interpreters[i].inter
			defer wg.Done()

			storageMap := mr.GetReadOnlyStorage().GetStorageMap(mr.Address, domain, false)
			storageMapValue := storageMap.ReadValue(&util.NopMemoryGauge{}, key)
			scm, err := getShardedCollectionMap(mr, storageMapValue)
			if err != nil {
				cancel(err)
				return
			}

			inter.SharedState.Config.InvalidatedResourceValidationEnabled = false

			for {
				select {
				case <-ctx.Done():
					return
				case keyPair, ok := <-keyPairChan:
					if !ok {
						return
					}

					ownedNFTs, err := getNftCollection(
						inter,
						keyPair.shardedCollectionKey,
						scm,
					)
					if err != nil {
						cancel(err)
						return
					}

					value, ok := ownedNFTs.Get(
						inter,
						interpreter.EmptyLocationRange,
						keyPair.nftCollectionKey,
					)
					if !ok {
						cancel(fmt.Errorf("expected value for key %s", keyPair.nftCollectionKey))
						return
					}

					var newValue interpreter.Value
					err = capturePanic(func() {
						newValue = value.Clone(inter)
					})
					if err != nil {
						cancel(err)
						return
					}
					if newValue == nil {
						cancel(fmt.Errorf("failed to clone value"))
						return
					}

					clonedValues = append(clonedValues,
						valueWithKeys{
							cricketKeyPair: keyPair,
							value:          newValue,
						},
					)

					ownedNFTs.UnsafeRemove(
						mr.Interpreter,
						interpreter.EmptyLocationRange,
						keyPair.nftCollectionKey,
					)

					progressLog(1)
				}
			}
		}(i)
	}
	// only after all values have been cloned, can they be set back
	wg.Wait()

	if ctx.Err() != nil {
		return nil, fmt.Errorf("context error when cloning values: %w", ctx.Err())
	}

	progressLog = util2.LogProgress(log, "closing child interpreters", len(interpreters))
	// close all child interpreters
	for _, i := range interpreters {
		err := i.close()
		if err != nil {
			return nil, err
		}
		progressLog(1)
	}

	log.Info().Msg("cloning empty cricket moments sharded collection")
	value = value.Clone(mr.Interpreter)
	log.Info().Msg("cloned empty cricket moments sharded collection")
	shardedCollectionMap, err = getShardedCollectionMap(mr, value)
	if err != nil {
		return nil, err
	}

	progressLog = util2.LogProgress(log, "setting cloned cricket moments", len(clonedValues))
	for _, clonedValue := range clonedValues {
		ownedNFTs, err := getNftCollection(
			mr.Interpreter,
			clonedValue.shardedCollectionKey,
			shardedCollectionMap,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to get nft collection: %w", err)
		}

		err = capturePanic(func() {
			ownedNFTs.UnsafeInsert(
				mr.Interpreter,
				interpreter.EmptyLocationRange,
				clonedValue.nftCollectionKey,
				clonedValue.value,
			)
		})
		if err != nil {
			return nil, fmt.Errorf("failed to set key: %w", err)
		}
		progressLog(1)
	}

	return value, nil
}

type hashWithKeys struct {
	key  uint64
	hash []byte
}

func recursiveStringShardedCollection(
	log zerolog.Logger,
	nWorkers int,
	mr *migratorRuntime,
	domain string,
	key interpreter.StorageMapKey,
	value interpreter.Value,
) ([]byte, error) {
	// hash all values
	hashes, err := hashAllNftsInAllCollections(
		log,
		nWorkers,
		mr,
		domain,
		key,
		value,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to hash all values: %w", err)
	}

	return consolidateAllHashes(
		log,
		nWorkers,
		hashes,
	)
}

func hashAllNftsInAllCollections(
	log zerolog.Logger,
	nWorkers int,
	mr *migratorRuntime,
	domain string,
	key interpreter.StorageMapKey,
	value interpreter.Value,
) (map[uint64]sortableHashes, error) {

	shardedCollectionMap, err := getShardedCollectionMap(mr, value)
	if err != nil {
		return nil, err
	}
	count, err := getCricketMomentsShardedCollectionNFTCount(
		mr,
		shardedCollectionMap)
	if err != nil {
		return nil, err
	}
	progressLog := util2.LogProgress(log, "hashing cricket moments", count)

	ctx, c := context.WithCancelCause(context.Background())
	cancel := func(err error) {
		if err != nil {
			log.Info().Err(err).Msg("canceling context")
		}
		c(err)
	}
	defer cancel(nil)

	keyPairChan := make(chan cricketKeyPair, count)
	hashChan := make(chan hashWithKeys, count)
	hashes := make(map[uint64]sortableHashes)
	wg := sync.WaitGroup{}
	wg.Add(nWorkers)
	done := make(chan struct{})

	// workers for hashing
	for i := 0; i < nWorkers; i++ {
		go func() {
			defer wg.Done()

			storageMap := mr.GetReadOnlyStorage().GetStorageMap(mr.Address, domain, false)
			storageMapValue := storageMap.ReadValue(&util.NopMemoryGauge{}, key)

			hashNFTWorker(progressLog, ctx, cancel, mr, storageMapValue, keyPairChan, hashChan)
		}()
	}

	// worker for collecting hashes
	go func() {
		defer close(done)
		defer log.Info().Msg("finished collecting hashes")
		for {
			select {
			case <-ctx.Done():
				return
			case hashed, ok := <-hashChan:
				if !ok {
					return
				}
				_, ok = hashes[hashed.key]
				if !ok {
					hashes[hashed.key] = make(sortableHashes, 0, 1_000_000)
				}
				hashes[hashed.key] = append(hashes[hashed.key], hashed.hash)
			}
		}
	}()

	// worker for dispatching values to hash
	go func() {
		defer close(keyPairChan)

		shardedCollectionMapIterator := shardedCollectionMap.Iterator()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			outerKey := shardedCollectionMapIterator.NextKey(nil)
			if outerKey == nil {
				break
			}

			ownedNFTs, err := getNftCollection(mr.Interpreter, outerKey, shardedCollectionMap)
			if err != nil {
				cancel(err)
				return
			}

			ownedNFTsIterator := ownedNFTs.Iterator()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				innerKey := ownedNFTsIterator.NextKey(nil)
				if innerKey == nil {
					break
				}

				keyPairChan <- cricketKeyPair{
					nftCollectionKey:     innerKey,
					shardedCollectionKey: outerKey,
				}
			}
		}
	}()

	wg.Wait()
	log.Info().Msg("finished hashing values")
	close(hashChan)
	<-done

	if ctx.Err() != nil {
		log.Info().Err(ctx.Err()).Msg("context error when hashing individual values")
		return nil, ctx.Err()
	}
	return hashes, nil
}

func consolidateAllHashes(
	log zerolog.Logger,
	nWorkers int,
	hashes map[uint64]sortableHashes,
) ([]byte, error) {

	log.Info().Msg("starting consolidateAllHashes")
	defer log.Info().Msg("done with consolidateAllHashes")

	ctx, c := context.WithCancelCause(context.Background())
	cancel := func(err error) {
		if err != nil {
			log.Info().Err(err).Msg("canceling context")
		}
		c(err)
	}
	defer cancel(nil)

	hashCollectionChan := make(chan uint64, nWorkers)
	hashedCollectionChan := make(chan []byte)
	finalHashes := make(sortableHashes, 0, 10_000_000)
	wg := sync.WaitGroup{}
	wg.Add(nWorkers)
	done := make(chan struct{})

	for i := 0; i < nWorkers; i++ {
		go func() {
			defer wg.Done()
			hasher := newHasher()

			for {
				select {
				case <-ctx.Done():
					return
				case i, ok := <-hashCollectionChan:
					if !ok {
						return
					}
					l := hashes[i]

					h, err := l.SortAndHash(hasher)
					if err != nil {
						cancel(fmt.Errorf("failed to write hash: %w", err))

						return
					}
					hashedCollectionChan <- h
				}
			}
		}()
	}

	go func() {
		defer close(done)
		defer log.Info().Msg("finished collecting final hashes")

		for {
			select {
			case <-ctx.Done():
				return
			case h, ok := <-hashedCollectionChan:
				if !ok {
					return
				}
				finalHashes = append(finalHashes, h)
			}
		}
	}()

	go func() {
		defer close(hashCollectionChan)
		defer log.Info().Msg("finished dispatching final hashes")

		for k := range hashes {
			select {
			case <-ctx.Done():
				return
			case hashCollectionChan <- k:
			}
		}
	}()

	wg.Wait()
	close(hashedCollectionChan)
	<-done

	if ctx.Err() != nil {
		log.Info().Err(ctx.Err()).Msg("context error when hashing values together")
		return nil, ctx.Err()
	}

	hasher := newHasher()
	h, err := finalHashes.SortAndHash(hasher)
	if err != nil {
		return nil, fmt.Errorf("failed to write hash: %w", err)
	}

	return h, nil
}

func hashNFTWorker(
	progress util2.LogProgressFunc,
	ctx context.Context,
	cancel context.CancelCauseFunc,
	mr *migratorRuntime,
	storageMapValue interpreter.Value,
	keyPairChan <-chan cricketKeyPair,
	hashChan chan<- hashWithKeys,
) {
	hasher := newHasher()

	shardedCollectionMap, err := getShardedCollectionMap(mr, storageMapValue)
	if err != nil {
		cancel(err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case keyPair, ok := <-keyPairChan:
			if !ok {
				return
			}
			s := ""

			ownedNFTs, err := getNftCollection(
				mr.Interpreter,
				keyPair.shardedCollectionKey,
				shardedCollectionMap,
			)
			if err != nil {
				cancel(err)
				return
			}

			value := ownedNFTs.GetKey(
				mr.Interpreter,
				interpreter.EmptyLocationRange,
				keyPair.nftCollectionKey,
			)

			err = capturePanic(func() {
				s = value.RecursiveString(interpreter.SeenReferences{})
			})
			if err != nil {
				cancel(err)
				return
			}
			hash := hasher.ComputeHash([]byte(s))
			hasher.Reset()

			uintKey := uint64(keyPair.shardedCollectionKey.(interpreter.UInt64Value))

			hashChan <- hashWithKeys{
				key:  uintKey,
				hash: hash,
			}
			progress(1)
		}
	}
}
