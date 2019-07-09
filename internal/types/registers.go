package types

import (
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type AccessMode int

const (
	CREATE AccessMode = iota
	SNAPSHOT_READ
	CONFIRMED_READ
	DEFERRED_WRITE
	READ_WRITE
)

type Type int

const (
	SIMPLE Type = iota
	CHUNK
	BALANCE
)

type KeyWeight struct {
	Key    []byte
	Weight float64
}

type Register struct {
	ID    []byte
	Value []byte
}

type TransactionRegister struct {
	Type       Type
	AccessMode AccessMode
	ID         []byte
	Keys       []KeyWeight
}

type IntermediateRegisters struct {
	TransactionHash crypto.Hash
	Registers       []Register
	ComputeUsed     uint64
}

func (m AccessMode) String() string {
	return [...]string{"CREATE", "SNAPSHOT_READ", "CONFIRMED_READ", "DEFERRED_WRITE", "READ_WRITE"}[m]
}

func (t Type) String() string {
	return [...]string{"SIMPLE", "CHUNK", "BALANCE"}[t]
}
