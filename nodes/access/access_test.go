package access

import (
	"context"
	"testing"
	"time"

	"github.com/dapperlabs/bamboo-emulator/data"
	"github.com/dapperlabs/bamboo-emulator/tests"
)

func TestCollectionBuilder(t *testing.T) {
	state := data.NewWorldState()

	transactionsIn := make(chan *data.Transaction)
	collectionsOut := make(chan *data.Collection)

	collectionBuilder := NewCollectionBuilder(state, transactionsIn, collectionsOut)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go collectionBuilder.Start(ctx, time.Millisecond)

	txA := tests.MockTransaction(1)
	txB := tests.MockTransaction(2)
	txC := tests.MockTransaction(2)

	transactionsIn <- txA
	transactionsIn <- txB
	transactionsIn <- txC

	collection := <-collectionsOut

	if len(collection.TransactionHashes) != 3 {
		t.Fatal("Collection does not contain all submitted transactions")
	}
}
