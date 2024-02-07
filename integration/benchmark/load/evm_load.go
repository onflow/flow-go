package load

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	evmTypes "github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/integration/benchmark/account"
	"github.com/onflow/flow-go/module/util"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"
	"io"
	"math/big"
	"time"
)

// eoa is a struct that represents an evm owned account.
type eoa struct {
	addressArg cadence.Value
	nonce      uint64
	pk         *ecdsa.PrivateKey
	adress     gethcommon.Address
}

type EVMTransferLoad struct {
	log               zerolog.Logger
	tokensPerTransfer cadence.UFix64

	eoaChan  chan *eoa
	doneChan chan struct{}

	transfers atomic.Uint64
	creations atomic.Uint64
}

func NewEVMTransferLoad(log zerolog.Logger) *EVMTransferLoad {
	load := &EVMTransferLoad{
		log:               log.With().Str("component", "EVMTransferLoad").Logger(),
		tokensPerTransfer: cadence.UFix64(100),
		// really large channel,
		// it's going to get filled as needed
		eoaChan:  make(chan *eoa, 1_000_000),
		doneChan: make(chan struct{}),
	}

	go load.reportStatus()

	return load
}

var _ Load = (*EVMTransferLoad)(nil)
var _ io.Closer = (*EVMTransferLoad)(nil)

func (l *EVMTransferLoad) Close() error {
	close(l.eoaChan)
	close(l.doneChan)
	return nil
}

func (l *EVMTransferLoad) reportStatus() {
	// report status every 10 seconds until done
	for {
		select {
		case <-l.doneChan:
			return
		case <-time.After(10 * time.Second):
			l.log.Info().
				Uint64("transfers", l.transfers.Load()).
				Uint64("creations", l.creations.Load()).
				Msg("EVMTransferLoad status report")
		}
	}

}

func (l *EVMTransferLoad) Type() LoadType {
	return EVMTransferLoadType
}

func (l *EVMTransferLoad) Setup(log zerolog.Logger, lc LoadContext) error {
	// create some EOA ahead of time to get a better result for the benchmark
	const createEOA = 3000

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(lc.Proposer.NumKeys())

	progress := util.LogProgress(l.log,
		util.DefaultLogProgressConfig(
			"creating and funding EOC accounts",
			createEOA,
		))

	l.log.Info().
		Int("number_of_accounts", createEOA).
		Int("number_of_keys", lc.Proposer.NumKeys()).
		Msg("creating and funding EOC accounts")

	for i := 0; i < createEOA; i += 1 {
		i := i
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			defer func() { progress(1) }()

			eoa, err := l.setupTransaction(log, lc)
			if err != nil {
				return err
			}

			if err != nil {
				l.log.
					Err(err).
					Int("index", i).
					Msg("error creating EOA accounts")
				return err
			}

			l.creations.Add(1)

			select {
			case l.eoaChan <- eoa:
			default:
			}

			return nil
		})
	}
	err := g.Wait()
	if err != nil {
		return fmt.Errorf("error creating EOC accounts: %w", err)
	}
	return nil
}

func (l *EVMTransferLoad) Load(log zerolog.Logger, lc LoadContext) error {
	select {
	case eoa := <-l.eoaChan:
		if eoa == nil {
			return nil
		}
		err := l.transferTransaction(log, lc, eoa)
		if err == nil {
			eoa.nonce += 1
		}

		l.transfers.Add(1)

		select {
		case l.eoaChan <- eoa:
		default:
		}

		return err
	default:
		// no eoa available, create a new one
		eoa, err := l.setupTransaction(log, lc)
		if err != nil {
			return err
		}
		l.creations.Add(1)

		select {
		case l.eoaChan <- eoa:
		default:
		}

		return nil
	}
}

func (l *EVMTransferLoad) setupTransaction(
	log zerolog.Logger,
	lc LoadContext,
) (*eoa, error) {
	eoa := &eoa{}

	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("error casting public key to ECDSA")
	}
	eoa.pk = privateKey
	eoa.adress = crypto.PubkeyToAddress(*publicKeyECDSA)

	addressCadenceBytes := make([]cadence.Value, 20)
	for i := range addressCadenceBytes {
		addressCadenceBytes[i] = cadence.UInt8(eoa.adress[i])
	}

	eoa.addressArg = cadence.NewArray(addressCadenceBytes).WithType(stdlib.EVMAddressBytesCadenceType)

	err = sendSimpleTransaction(
		log,
		lc,
		func(
			log zerolog.Logger,
			lc LoadContext,
			acc *account.FlowAccount,
		) (*flowsdk.Transaction, error) {
			sc := systemcontracts.SystemContractsForChain(lc.ChainID)

			amountArg, err := cadence.NewUFix64("1.0")
			if err != nil {
				return nil, err
			}

			// Fund evm address
			txBody := flowsdk.NewTransaction().
				SetScript([]byte(fmt.Sprintf(
					`
						import EVM from %s
						import FungibleToken from %s
						import FlowToken from %s

						transaction(address: [UInt8; 20], amount: UFix64) {
							let fundVault: @FlowToken.Vault

							prepare(signer: AuthAccount) {
								let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
									?? panic("Could not borrow reference to the owner's Vault!")
						
								// 1.0 Flow for the EVM gass fees
								self.fundVault <- vaultRef.withdraw(amount: amount+1.0) as! @FlowToken.Vault
							}

							execute {
								let acc <- EVM.createBridgedAccount()
								acc.deposit(from: <-self.fundVault)
								let fundAddress = EVM.EVMAddress(bytes: address)
								acc.call(
										to: fundAddress,
										data: [],
										gasLimit: 21000,
										value: EVM.Balance(flow: amount))

								destroy acc
							}
						}
					`,
					sc.FlowServiceAccount.Address.HexWithPrefix(),
					sc.FungibleToken.Address.HexWithPrefix(),
					sc.FlowToken.Address.HexWithPrefix(),
				)))

			err = txBody.AddArgument(eoa.addressArg)
			if err != nil {
				return nil, err
			}
			err = txBody.AddArgument(amountArg)
			if err != nil {
				return nil, err
			}

			return txBody, nil
		})
	if err != nil {
		return nil, err
	}
	return eoa, nil
}

func (l *EVMTransferLoad) transferTransaction(
	log zerolog.Logger,
	lc LoadContext,
	eoa *eoa,
) error {
	return sendSimpleTransaction(
		log,
		lc,
		func(
			log zerolog.Logger,
			lc LoadContext,
			acc *account.FlowAccount,
		) (*flowsdk.Transaction, error) {
			nonce := eoa.nonce
			to := gethcommon.HexToAddress("")
			gasPrice := big.NewInt(0)

			oneFlow := cadence.UFix64(100_000_000)
			amount := new(big.Int).Div(evmTypes.OneFlowBalance, big.NewInt(int64(oneFlow)))
			evmTx := types.NewTx(&types.LegacyTx{Nonce: nonce, To: &to, Value: amount, Gas: params.TxGas, GasPrice: gasPrice, Data: nil})

			signed, err := types.SignTx(evmTx, emulator.GetDefaultSigner(), eoa.pk)
			if err != nil {
				return nil, fmt.Errorf("error signing EVM transaction: %w", err)
			}
			var encoded bytes.Buffer
			err = signed.EncodeRLP(&encoded)
			if err != nil {
				return nil, fmt.Errorf("error encoding EVM transaction: %w", err)
			}

			encodedCadence := make([]cadence.Value, 0)
			for _, b := range encoded.Bytes() {
				encodedCadence = append(encodedCadence, cadence.UInt8(b))
			}
			transactionBytes := cadence.NewArray(encodedCadence).WithType(stdlib.EVMTransactionBytesCadenceType)

			sc := systemcontracts.SystemContractsForChain(lc.ChainID)
			txBody := flowsdk.NewTransaction().
				SetScript([]byte(fmt.Sprintf(
					`
						import EVM from %s
						import FungibleToken from %s
						import FlowToken from %s
						
						transaction(encodedTx: [UInt8]) {
							//let bridgeVault: @FlowToken.Vault
						
							prepare(signer: AuthAccount){}
						
							execute {
								let feeAcc <- EVM.createBridgedAccount()
								EVM.run(tx: encodedTx, coinbase: feeAcc.address())
								destroy feeAcc
							}
						}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
					sc.FungibleToken.Address.HexWithPrefix(),
					sc.FlowToken.Address.HexWithPrefix(),
				)))

			err = txBody.AddArgument(transactionBytes)
			if err != nil {
				return nil, fmt.Errorf("error adding argument to transaction: %w", err)
			}
			return txBody, nil
		})
}
