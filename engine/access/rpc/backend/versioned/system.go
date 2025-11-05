package versioned

import (
	"fmt"
	"maps"
	"slices"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/model/flow"
)

type SystemCollection interface {
	ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error)
	ExecuteCallbacksTransactions(chain flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error)
	SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error)
	SystemCollection(chain flow.Chain, processEvents flow.EventsList) (*flow.Collection, error)
}

type SystemCollectionV2 struct{}

func (s *SystemCollectionV2) ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	return blueprints.ProcessCallbacksTransaction(chain)
}

func (s *SystemCollectionV2) ExecuteCallbacksTransactions(chain flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error) {
	return blueprints.ExecuteCallbacksTransactions(chain, processEvents)
}

func (s *SystemCollectionV2) SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	return blueprints.SystemChunkTransaction(chain)
}

func (s *SystemCollectionV2) SystemCollection(chain flow.Chain, processEvents flow.EventsList) (*flow.Collection, error) {
	return blueprints.SystemCollection(chain, processEvents)
}

type SystemCollectionV1 struct{}

func (s *SystemCollectionV1) ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	return blueprints.ProcessCallbacksTransaction(chain)
}

func (s *SystemCollectionV1) ExecuteCallbacksTransactions(chain flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error) {
	// todo change to blueprints.ExecuteCallbacksTransactionsV0()
	return blueprints.ExecuteCallbacksTransactions(chain, processEvents)
}

func (s *SystemCollectionV1) SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	return blueprints.SystemChunkTransaction(chain)
}

func (s *SystemCollectionV1) SystemCollection(chain flow.Chain, processEvents flow.EventsList) (*flow.Collection, error) {
	return blueprints.SystemCollection(chain, processEvents)
}

type SystemCollectionV0 struct{}

func (s *SystemCollectionV0) SystemCollection(chain flow.Chain, processEvents flow.EventsList) (*flow.Collection, error) {
	systemTx, err := s.SystemChunkTransaction(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to construct system chunk transaction: %w", err)
	}

	return flow.NewCollection(flow.UntrustedCollection{
		Transactions: []*flow.TransactionBody{systemTx},
	})
}

func (s *SystemCollectionV0) SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	return blueprints.SystemChunkTransaction(chain)
}

func (s *SystemCollectionV0) ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error) {
	return nil, nil
}

func (s *SystemCollectionV0) ExecuteCallbacksTransactions(chain flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error) {
	return nil, nil
}

type VersionMapper interface {
	GetVersion(height uint64) (string, error)
	GetBoundary(version string) (uint64, error)
}

type StaticVersionMapper struct {
	versionBoundaries map[uint64]string
}

func NewStaticVersionMapper(versionBoundaries map[uint64]string) *StaticVersionMapper {
	return &StaticVersionMapper{
		versionBoundaries: versionBoundaries,
	}
}

func (s *StaticVersionMapper) GetVersion(height uint64) (string, error) {
	boundaries := slices.Collect(maps.Keys(s.versionBoundaries))
	slices.Sort(boundaries)

	for _, boundary := range boundaries {
		if height >= boundary {
			return s.versionBoundaries[boundary], nil
		}
	}

	return "", fmt.Errorf("height not found: %d", height)
}

func (s *StaticVersionMapper) GetBoundary(version string) (uint64, error) {
	for b, v := range s.versionBoundaries {
		if v == version {
			return b, nil
		}
	}

	return 0, fmt.Errorf("version not found: %s", version)
}

type Versioned[T any] struct {
	versionedTypes map[string]T
	versionMapper  VersionMapper
}

func NewVersioned[T any](versionedTypes map[string]T, versionMapper VersionMapper) (*Versioned[T], error) {
	for _, v := range slices.Collect(maps.Keys(versionedTypes)) {
		_, err := versionMapper.GetBoundary(v)
		if err != nil {
			return nil, fmt.Errorf("missing version: %w", err)
		}
	}

	return &Versioned[T]{
		versionedTypes: versionedTypes,
		versionMapper:  versionMapper,
	}, nil
}

func (v *Versioned[T]) Get(height uint64) T {
	version, err := v.versionMapper.GetVersion(height)
	t, ok := v.versionedTypes[version]
	if err != nil || !ok {
		return v.versionedTypes["latest"] // latest
	}

	return t
}

func (v *Versioned[T]) all() []T {
	return slices.Collect(maps.Values(v.versionedTypes))
}

type StaticSystemCollection struct {
	transactions []*flow.TransactionBody
}

func NewStaticSystemCollection(chain flow.Chain, versionedCollection Versioned[SystemCollection]) (*StaticSystemCollection, error) {
	transactions := make([]*flow.TransactionBody, 0)

	for _, collection := range versionedCollection.all() {
		collection, err := collection.SystemCollection(chain, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to construct system collection: %w", err)
		}

		transactions = append(transactions, collection.Transactions...)
	}

	return &StaticSystemCollection{
		transactions: transactions,
	}, nil
}

func (s *StaticSystemCollection) ByID(id flow.Identifier) (*flow.TransactionBody, bool) {
	for _, tx := range s.transactions {
		if tx.ID() == id {
			return tx, true
		}
	}
	return nil, false
}
