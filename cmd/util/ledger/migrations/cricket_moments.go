package migrations

import (
	"fmt"

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
	return 0, nil
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

func getNftCollection(inter *interpreter.Interpreter, outerKey interpreter.Value, shardedCollectionMap *interpreter.DictionaryValue) (*interpreter.DictionaryValue, error) {
	value := shardedCollectionMap.GetKey(
		inter,
		interpreter.EmptyLocationRange,
		outerKey,
	)

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
