package subscription

import (
	"context"
	"fmt"
)

// DataProvider represents a data provider
// TODO: can and should be generic rather than return `any`. though it requires lots of refactoring,
// so it'll be done in a PR with the subscription package refactoring.
type DataProvider interface {
	NextData(ctx context.Context) (any, error)
}

// HeightByFuncProvider is a DataProvider that uses a GetDataByHeightFunc
// and internal height counter to sequentially fetch data by height.
type HeightByFuncProvider struct {
	nextHeight uint64
	getData    GetDataByHeightFunc
}

func NewHeightByFuncProvider(startHeight uint64, getData GetDataByHeightFunc) *HeightByFuncProvider {
	return &HeightByFuncProvider{
		nextHeight: startHeight,
		getData:    getData,
	}
}

var _ DataProvider = (*HeightByFuncProvider)(nil)

func (p *HeightByFuncProvider) NextData(ctx context.Context) (any, error) {
	v, err := p.getData(ctx, p.nextHeight)
	if err != nil {
		return nil, fmt.Errorf("could not get data for height %d: %w", p.nextHeight, err)
	}
	p.nextHeight++
	return v, nil
}
