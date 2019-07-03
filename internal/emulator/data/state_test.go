package data

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

func TestUpdateTransactionStatus(t *testing.T) {
	s := NewWorldState(logrus.New())

	txA := &Transaction{
		ToAddress:      crypto.BytesToAddress([]byte{}),
		Script:         []byte{},
		Nonce:          1,
		ComputeLimit:   10,
		ComputeUsed:    0,
		PayerSignature: []byte{},
		Status:         TxPending,
	}

	s.InsertTransaction(txA)
	s.UpdateTransactionStatus(txA.Hash(), TxFinalized)

	txB, err := s.GetTransaction(txA.Hash())
	if err != nil {
		t.Fail()
	}

	if txB.Status != TxFinalized {
		t.Fail()
	}

	if txA.Status == TxFinalized {
		t.Fail()
	}
}

func TestSealBlock(t *testing.T) {
	s := NewWorldState(logrus.New())

	blockA := &Block{
		Number:            1,
		Timestamp:         time.Now(),
		PrevBlockHash:     crypto.ZeroBlockHash(),
		Status:            BlockPending,
		CollectionHashes:  []crypto.Hash{},
		TransactionHashes: []crypto.Hash{},
	}

	s.AddBlock(blockA)
	s.SealBlock(blockA.Hash())

	blockB, err := s.GetBlockByHash(blockA.Hash())
	if err != nil {
		t.Fail()
	}

	if blockB.Status != BlockSealed {
		t.Fail()
	}

	if blockA.Status == BlockSealed {
		t.Fail()
	}
}
