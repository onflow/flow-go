package account

import (
	"context"
	"errors"
	"fmt"
	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/module/util"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/integration/benchmark/common"
	"github.com/onflow/flow-go/integration/benchmark/scripts"
	"github.com/onflow/flow-go/model/flow"
)

var ErrNoAccountsAvailable = errors.New("no accounts available")

type AccountProvider interface {
	// BorrowAvailableAccount borrows an account from the account provider.
	// It doesn't block.
	// If no account is available, it returns ErrNoAccountsAvailable.
	BorrowAvailableAccount() (*FlowAccount, error)
	// ReturnAvailableAccount returns an account to the account provider, so it can be reused.
	ReturnAvailableAccount(*FlowAccount)
}

type provider struct {
	log                      zerolog.Logger
	availableAccounts        chan *FlowAccount
	numberOfAccounts         int
	accountCreationBatchSize int
}

var _ AccountProvider = (*provider)(nil)

func (p *provider) BorrowAvailableAccount() (*FlowAccount, error) {
	select {
	case account := <-p.availableAccounts:
		return account, nil
	default:
		return nil, ErrNoAccountsAvailable
	}
}

func (p *provider) ReturnAvailableAccount(account *FlowAccount) {
	select {
	case p.availableAccounts <- account:
	default:
	}
}

func SetupProvider(
	log zerolog.Logger,
	ctx context.Context,
	numberOfAccounts int,
	fundAmount uint64,
	rb common.ReferenceBlockProvider,
	creator *FlowAccount,
	sender common.TransactionSender,
	chain flow.Chain,
) (AccountProvider, error) {
	p := &provider{
		log:                      log.With().Str("component", "AccountProvider").Logger(),
		availableAccounts:        make(chan *FlowAccount, numberOfAccounts),
		numberOfAccounts:         numberOfAccounts,
		accountCreationBatchSize: 750,
	}

	err := p.init(ctx, fundAmount, rb, creator, sender, chain)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize account provider: %w", err)
	}

	return p, nil
}

func (p *provider) init(
	ctx context.Context,
	fundAmount uint64,
	rb common.ReferenceBlockProvider,
	creator *FlowAccount,
	sender common.TransactionSender,
	chain flow.Chain,
) error {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(creator.NumKeys())

	progress := util.LogProgress(p.log,
		util.DefaultLogProgressConfig(
			"creating accounts",
			p.numberOfAccounts,
		))

	p.log.Info().
		Int("number_of_accounts", p.numberOfAccounts).
		Int("account_creation_batch_size", p.accountCreationBatchSize).
		Int("number_of_keys", creator.NumKeys()).
		Msg("creating accounts")

	for i := 0; i < p.numberOfAccounts; i += p.accountCreationBatchSize {
		i := i
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return nil
			default:
			}

			num := p.accountCreationBatchSize
			if i+p.accountCreationBatchSize > p.numberOfAccounts {
				num = p.numberOfAccounts - i
			}

			defer func() { progress(num) }()

			err := p.createAccountBatch(num, fundAmount, rb, creator, sender, chain)
			if err != nil {
				p.log.
					Err(err).
					Int("batch_size", num).
					Int("index", i).
					Msg("error creating accounts")
				return err
			}

			return nil
		})
	}
	err := g.Wait()
	if err != nil {
		return fmt.Errorf("error creating accounts: %w", err)
	}
	return nil
}

func (p *provider) createAccountBatch(
	num int,
	fundAmount uint64,
	rb common.ReferenceBlockProvider,
	creator *FlowAccount,
	sender common.TransactionSender,
	chain flow.Chain,
) error {
	wrapErr := func(err error) error {
		return fmt.Errorf("error in create accounts: %w", err)
	}

	privKey := RandomPrivateKey()
	accountKey := flowsdk.NewAccountKey().
		FromPrivateKey(privKey).
		SetHashAlgo(crypto.SHA3_256).
		SetWeight(flowsdk.AccountKeyWeightThreshold)

	sc := systemcontracts.SystemContractsForChain(chain.ChainID())

	// Generate an account creation script
	createAccountTx := flowsdk.NewTransaction().
		SetScript(scripts.CreateAccountsTransaction(
			flowsdk.BytesToAddress(sc.FungibleToken.Address.Bytes()),
			flowsdk.BytesToAddress(sc.FlowToken.Address.Bytes()))).
		SetReferenceBlockID(rb.ReferenceBlockID()).
		SetComputeLimit(999999)

	publicKey := blueprints.BytesToCadenceArray(accountKey.PublicKey.Encode())
	count := cadence.NewInt(num)

	initialTokenAmount := cadence.UFix64(fundAmount)

	err := createAccountTx.AddArgument(publicKey)
	if err != nil {
		return wrapErr(err)
	}

	err = createAccountTx.AddArgument(count)
	if err != nil {
		return wrapErr(err)
	}

	err = createAccountTx.AddArgument(initialTokenAmount)
	if err != nil {
		return wrapErr(err)
	}

	key, err := creator.GetKey()
	if err != nil {
		return wrapErr(err)
	}
	defer key.Done()

	err = key.SetProposerPayerAndSign(createAccountTx)
	if err != nil {
		return wrapErr(err)
	}

	result, err := sender.Send(createAccountTx)
	if err == nil || errors.Is(err, common.TransactionError{}) {
		key.IncrementSequenceNumber()
	}
	if err != nil {
		return wrapErr(err)
	}

	var accountsCreated int
	for _, event := range result.Events {
		if event.Type != flowsdk.EventAccountCreated {
			continue
		}

		accountCreatedEvent := flowsdk.AccountCreatedEvent(event)
		accountAddress := accountCreatedEvent.Address()

		newAcc, err := New(accountAddress, privKey, crypto.SHA3_256, []*flowsdk.AccountKey{accountKey})
		if err != nil {
			return fmt.Errorf("failed to create account: %w", err)
		}
		accountsCreated++

		p.availableAccounts <- newAcc
	}
	if accountsCreated != num {
		return fmt.Errorf("failed to create enough contracts, expected: %d, created: %d",
			num, accountsCreated)
	}
	return nil
}
