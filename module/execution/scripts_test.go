package execution

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/onflow/flow-go/fvm/errors"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"
	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/engine/execution/computation/query/mock"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/storage"
	pebbleStorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestScripts(t *testing.T) {
	suite.Run(t, new(scriptTestSuite))
}

type scriptTestSuite struct {
	suite.Suite
	scripts       *Scripts
	registerIndex storage.RegisterIndex
	vm            *fvm.VirtualMachine
	vmCtx         fvm.Context
	chain         flow.Chain
	height        uint64
	snapshot      snapshot.SnapshotTree
	dbDir         string
}

func (s *scriptTestSuite) TestScriptExecution() {
	s.Run("Simple Script Execution", func() {
		number := int64(42)
		code := []byte(fmt.Sprintf("access(all) fun main(): Int { return %d; }", number))

		result, err := s.scripts.ExecuteAtBlockHeight(context.Background(), code, nil, s.height)
		s.Require().NoError(err)
		val, err := jsoncdc.Decode(nil, result)
		s.Require().NoError(err)
		s.Assert().Equal(number, val.(cadence.Int).Value.Int64())
	})

	s.Run("Get Block", func() {
		code := []byte(fmt.Sprintf(`access(all) fun main(): UInt64 {
			getBlock(at: %d)!
			return getCurrentBlock().height 
		}`, s.height))

		result, err := s.scripts.ExecuteAtBlockHeight(context.Background(), code, nil, s.height)
		s.Require().NoError(err)
		val, err := jsoncdc.Decode(nil, result)
		s.Require().NoError(err)
		// make sure that the returned block height matches the current one set
		s.Assert().Equal(s.height, val.(cadence.UInt64).ToGoValue())
	})

	s.Run("Handle not found Register", func() {
		// use a non-existing address to trigger register get function
		code := []byte("import Foo from 0x01; access(all) fun main() { }")

		result, err := s.scripts.ExecuteAtBlockHeight(context.Background(), code, nil, s.height)
		s.Assert().Error(err)
		s.Assert().Nil(result)
	})

	s.Run("Valid Argument", func() {
		code := []byte("access(all) fun main(foo: Int): Int { return foo }")
		arg := cadence.NewInt(2)
		encoded, err := jsoncdc.Encode(arg)
		s.Require().NoError(err)

		result, err := s.scripts.ExecuteAtBlockHeight(
			context.Background(),
			code,
			[][]byte{encoded},
			s.height,
		)
		s.Require().NoError(err)
		s.Assert().Equal(encoded, result)
	})

	s.Run("Invalid Argument", func() {
		code := []byte("access(all) fun main(foo: Int): Int { return foo }")
		invalid := [][]byte{[]byte("i")}

		result, err := s.scripts.ExecuteAtBlockHeight(context.Background(), code, invalid, s.height)
		s.Assert().Nil(result)
		var coded errors.CodedError
		s.Require().True(errors.As(err, &coded))
		fmt.Println(coded.Code(), coded.Error())
		s.Assert().Equal(errors.ErrCodeInvalidArgumentError, coded.Code())
	})
}

func (s *scriptTestSuite) TestGetAccount() {
	s.Run("Get Service Account", func() {
		address := s.chain.ServiceAddress()
		account, err := s.scripts.GetAccountAtBlockHeight(context.Background(), address, s.height)
		s.Require().NoError(err)
		s.Assert().Equal(address, account.Address)
		s.Assert().NotZero(account.Balance)
		s.Assert().NotZero(len(account.Contracts))
	})

	s.Run("Get New Account", func() {
		address := s.createAccount()
		account, err := s.scripts.GetAccountAtBlockHeight(context.Background(), address, s.height)
		s.Require().NoError(err)
		s.Require().Equal(address, account.Address)
		s.Assert().Zero(account.Balance)
	})
}

func (s *scriptTestSuite) SetupTest() {
	logger := unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	entropyProvider := testutil.EntropyProviderFixture(nil)
	blockchain := unittest.BlockchainFixture(10)
	headers := newBlockHeadersStorage(blockchain)

	s.chain = flow.Emulator.Chain()
	s.snapshot = snapshot.NewSnapshotTree(nil)
	s.vm = fvm.NewVirtualMachine()
	s.vmCtx = fvm.NewContext(
		fvm.WithChain(s.chain),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
	)
	s.height = blockchain[0].Header.Height

	entropyBlock := mock.NewEntropyProviderPerBlock(s.T())
	entropyBlock.
		On("AtBlockID", mocks.AnythingOfType("flow.Identifier")).
		Return(entropyProvider).
		Maybe()

	s.dbDir = unittest.TempDir(s.T())
	db := pebbleStorage.NewBootstrappedRegistersWithPathForTest(s.T(), s.dbDir, s.height, s.height)
	pebbleRegisters, err := pebbleStorage.NewRegisters(db)
	s.Require().NoError(err)
	s.registerIndex = pebbleRegisters

	index, err := indexer.New(logger, metrics.NewNoopCollector(), nil, s.registerIndex, headers, nil, nil)
	s.Require().NoError(err)

	scripts, err := NewScripts(
		logger,
		metrics.NewNoopCollector(),
		s.chain.ChainID(),
		entropyBlock,
		headers,
		index.RegisterValue,
		query.NewDefaultConfig(),
	)
	s.Require().NoError(err)
	s.scripts = scripts

	s.bootstrap()
}

func (s *scriptTestSuite) TearDownTest() {
	s.Require().NoError(os.RemoveAll(s.dbDir))
}

func (s *scriptTestSuite) bootstrap() {
	bootstrapOpts := []fvm.BootstrapProcedureOption{
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
	}

	executionSnapshot, out, err := s.vm.Run(
		s.vmCtx,
		fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOpts...),
		s.snapshot)

	s.Require().NoError(err)
	s.Require().NoError(out.Err)

	s.height++
	err = s.registerIndex.Store(executionSnapshot.UpdatedRegisters(), s.height)
	s.Require().NoError(err)

	s.snapshot = s.snapshot.Append(executionSnapshot)
}

func (s *scriptTestSuite) createAccount() flow.Address {
	const createAccountTransaction = `
		transaction {
		  prepare(signer: auth(Storage, Capabilities) &Account) {
			let account = Account(payer: signer)
		  }
		}`

	txBody := flow.NewTransactionBody().
		SetScript([]byte(createAccountTransaction)).
		AddAuthorizer(s.chain.ServiceAddress())

	executionSnapshot, output, err := s.vm.Run(
		s.vmCtx,
		fvm.Transaction(txBody, 0),
		s.snapshot,
	)
	s.Require().NoError(err)
	s.Require().NoError(output.Err)

	s.height++
	err = s.registerIndex.Store(executionSnapshot.UpdatedRegisters(), s.height)
	s.Require().NoError(err)

	s.snapshot = s.snapshot.Append(executionSnapshot)

	var accountCreatedEvents []flow.Event
	for _, event := range output.Events {
		if event.Type != flow.EventAccountCreated {
			continue
		}
		accountCreatedEvents = append(accountCreatedEvents, event)
		break
	}
	s.Require().Len(accountCreatedEvents, 1)

	data, err := ccf.Decode(nil, accountCreatedEvents[0].Payload)
	s.Require().NoError(err)
	address := flow.ConvertAddress(data.(cadence.Event).Fields[0].(cadence.Address))

	return address
}

func newBlockHeadersStorage(blocks []*flow.Block) storage.Headers {
	blocksByHeight := make(map[uint64]*flow.Block)
	for _, b := range blocks {
		blocksByHeight[b.Header.Height] = b
	}

	return synctest.MockBlockHeaderStorage(synctest.WithByHeight(blocksByHeight))
}
