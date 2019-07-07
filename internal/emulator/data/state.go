package data

import (
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

// WorldState represents the current state of the blockchain.
type WorldState struct {
	accounts          map[crypto.Address]crypto.Account
	accountsMutex     sync.RWMutex
	blocks            map[crypto.Hash]Block
	blocksMutex       sync.RWMutex
	collections       map[crypto.Hash]Collection
	collectionsMutex  sync.RWMutex
	transactions      map[crypto.Hash]Transaction
	transactionsMutex sync.RWMutex
	registers         map[string][]byte
	registersMutex    sync.RWMutex
	blockchain        []crypto.Hash
	blockchainMutex   sync.RWMutex
}

// NewWorldState instantiates a new state object with a genesis block and root account.
func NewWorldState(log *logrus.Logger) (*WorldState, error) {
	accounts := make(map[crypto.Address]crypto.Account)
	blocks := make(map[crypto.Hash]Block)
	collections := make(map[crypto.Hash]Collection)
	txs := make(map[crypto.Hash]Transaction)
	registers := make(map[string][]byte)

	mnemonic, err := crypto.LoadMnemonic()
	if err != nil {
		return nil, err
	}

	log.WithFields(logrus.Fields{
		"filename": crypto.MnemonicFile,
	}).Debugf(
		"Loading mnemonic file from: %s",
		crypto.MnemonicFile,
	)

	wallet, err := crypto.GenWalletFromMnemonic(mnemonic)
	if err != nil {
		return nil, err
	}

	log.WithFields(logrus.Fields{
		"mnemonic": wallet.Mnemonic,
	}).Infof(
		"Generating wallet from mneumonic: %s",
		wallet.Mnemonic,
	)

	root, err := wallet.CreateRootAccount()
	if err != nil {
		return nil, err
	}

	accounts[root.Address] = *root

	log.WithFields(logrus.Fields{
		"address": root.Address,
		"balance": root.Balance,
		"path":    root.Path,
	}).Debugf(
		"Creating root account %v from derivation path %s",
		root.Address,
		root.Path,
	)

	genesis := MintGenesisBlock()

	chain := []crypto.Hash{genesis.Hash()}
	blocks[genesis.Hash()] = *genesis

	log.WithFields(logrus.Fields{
		"blockNum":        genesis.Number,
		"blockHash":       genesis.Hash(),
		"numCollections":  0,
		"numTransactions": 0,
	}).Infof(
		"Minting genesis block (0x%v)",
		genesis.Hash(),
	)

	return &WorldState{
		accounts:     accounts,
		blocks:       blocks,
		collections:  collections,
		transactions: txs,
		registers:    registers,
		blockchain:   chain,
	}, nil
}

// GetLatestBlock gets the most recent block in the blockchain.
func (s *WorldState) GetLatestBlock() *Block {
	s.blockchainMutex.RLock()
	currHeight := len(s.blockchain)
	blockHash := s.blockchain[currHeight-1]
	s.blockchainMutex.RUnlock()

	block, _ := s.GetBlockByHash(blockHash)
	return block
}

// GetBlockByNumber gets a block by number.
func (s *WorldState) GetBlockByNumber(n uint64) (*Block, error) {
	s.blockchainMutex.RLock()
	currHeight := len(s.blockchain)

	if int(n) < currHeight {
		blockHash := s.blockchain[n]
		s.blockchainMutex.RUnlock()
		return s.GetBlockByHash(blockHash)
	}

	s.blockchainMutex.RUnlock()

	return nil, &InvalidBlockNumberError{blockNumber: n}
}

// GetBlockByHash gets a block by hash.
func (s *WorldState) GetBlockByHash(h crypto.Hash) (*Block, error) {
	s.blocksMutex.RLock()
	defer s.blocksMutex.RUnlock()

	if block, ok := s.blocks[h]; ok {
		return &block, nil
	}

	return nil, &ItemNotFoundError{hash: h}
}

// GetCollection gets a collection by hash.
func (s *WorldState) GetCollection(h crypto.Hash) (*Collection, error) {
	s.collectionsMutex.RLock()
	defer s.collectionsMutex.RUnlock()

	if collection, ok := s.collections[h]; ok {
		return &collection, nil
	}

	return nil, &ItemNotFoundError{hash: h}
}

// GetTransaction gets a transaction by hash.
func (s *WorldState) GetTransaction(h crypto.Hash) (*Transaction, error) {
	s.transactionsMutex.RLock()
	defer s.transactionsMutex.RUnlock()

	if tx, ok := s.transactions[h]; ok {
		return &tx, nil
	}

	return nil, &ItemNotFoundError{hash: h}
}

// GetAccount gets an account by address.
func (s *WorldState) GetAccount(a crypto.Address) (*crypto.Account, error) {
	s.accountsMutex.RLock()
	defer s.accountsMutex.RUnlock()

	if account, ok := s.accounts[a]; ok {
		return &account, nil
	}

	return nil, &ItemNotFoundError{}
}

// GetRegister gets a register by ID.
func (s *WorldState) GetRegister(id string) []byte {
	s.registersMutex.RLock()
	defer s.registersMutex.RUnlock()

	return s.registers[id]
}

// AddBlock adds a new block to the blockchain.
func (s *WorldState) AddBlock(block *Block) error {
	s.blocksMutex.Lock()
	defer s.blocksMutex.Unlock()
	if _, exists := s.blocks[block.Hash()]; exists {
		return &DuplicateItemError{hash: block.Hash()}
	}

	s.blocks[block.Hash()] = *block

	s.blockchainMutex.Lock()
	s.blockchain = append(s.blockchain, block.Hash())
	s.blockchainMutex.Unlock()

	return nil
}

// InsertCollection inserts a new collection into the state.
func (s *WorldState) InsertCollection(col *Collection) error {
	s.collectionsMutex.Lock()
	defer s.collectionsMutex.Unlock()

	if _, exists := s.collections[col.Hash()]; exists {
		return &DuplicateItemError{hash: col.Hash()}
	}

	s.collections[col.Hash()] = *col

	return nil
}

// InsertTransaction inserts a new transaction into the state.
func (s *WorldState) InsertTransaction(tx *Transaction) error {
	s.transactionsMutex.Lock()
	defer s.transactionsMutex.Unlock()

	if _, exists := s.transactions[tx.Hash()]; exists {
		return &DuplicateItemError{hash: tx.Hash()}
	}

	s.transactions[tx.Hash()] = *tx

	return nil
}

// InsertAccount adds a newly created account into the world state.
func (s *WorldState) InsertAccount(account *crypto.Account) error {
	s.accountsMutex.Lock()
	defer s.accountsMutex.Unlock()

	if _, exists := s.accounts[account.Address]; exists {
		return &DuplicateAccountError{address: account.Address}
	}

	s.accounts[account.Address] = *account

	return nil
}

// CommitRegisters updates the register state with the values of a register map.
func (s *WorldState) CommitRegisters(registers Registers) {
	s.registersMutex.Lock()

	for id, value := range registers {
		s.registers[id] = value
	}

	s.registersMutex.Unlock()
}

// UpdateTransactionStatus updates the status of a single transaction.
func (s *WorldState) UpdateTransactionStatus(h crypto.Hash, status TxStatus) error {
	tx, err := s.GetTransaction(h)
	if err != nil {
		return err
	}

	s.transactionsMutex.Lock()
	tx.Status = status
	s.transactions[tx.Hash()] = *tx
	s.transactionsMutex.Unlock()

	return nil
}

// SealBlock seals a block on the blockchain.
func (s *WorldState) SealBlock(h crypto.Hash) error {
	block, err := s.GetBlockByHash(h)
	if err != nil {
		return err
	}

	s.blocksMutex.Lock()
	block.Status = BlockSealed
	s.blocks[block.Hash()] = *block
	s.blocksMutex.Unlock()

	return nil
}
