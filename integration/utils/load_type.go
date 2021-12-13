package utils

import (
	"fmt"
	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	"strings"
)

type WorkLoad interface {
	Work(lg *ContLoadGenerator) (work func(workerID int))
}

type SimpleWorkLoad struct {
	Name         string
	Script       func(lg *ContLoadGenerator, workerID int, acc *flowAccount) ([]byte, error)
	AddArguments func(lg *ContLoadGenerator, workerID int, tx *flowsdk.Transaction) error
}

func (c *SimpleWorkLoad) Work(lg *ContLoadGenerator) (work func(workerID int)) {
	log := lg.log.With().Str("workload", c.Name).Logger()

	return func(workerID int) {
		blockRef, err := lg.blockRef.Get()
		if err != nil {
			log.Error().Err(err).Msgf("error getting reference block")
			return
		}

		log.Trace().Msgf("getting next available account")
		acc := <-lg.availableAccounts
		defer func() { lg.availableAccounts <- acc }()

		script, err := c.Script(lg, workerID, acc)
		if err != nil {
			log.Error().Err(err).Msgf("error generating transaction script")
			return
		}

		log.Trace().Msgf("creating transaction")
		tx := flowsdk.NewTransaction().
			SetReferenceBlockID(blockRef).
			SetScript(script).
			SetGasLimit(9999).
			SetProposalKey(*acc.address, 0, acc.seqNumber).
			SetPayer(*acc.address).
			AddAuthorizer(*acc.address)

		if c.AddArguments != nil {
			err = c.AddArguments(lg, workerID, tx)
			if err != nil {
				log.Error().Err(err).Msgf("error adding arguments to the transaction")
				return
			}
		}

		log.Trace().Msgf("signing transaction")
		err = acc.signTx(tx, 0)
		if err != nil {
			log.Error().Err(err).Msgf("error signing transaction")
			return
		}

		lg.sendTx(tx)
	}
}

var _ WorkLoad = &SimpleWorkLoad{}

var (
	sendTokenTransferWorkload = &SimpleWorkLoad{
		Name: "TokenTransfer",
		Script: func(lg *ContLoadGenerator, workerID int, acc *flowAccount) ([]byte, error) {
			lg.log.Trace().Msgf("getting next account")
			nextAcc := lg.accounts[(acc.i+1)%len(lg.accounts)]

			return TokenTransferScript(
				lg.fungibleTokenAddress,
				lg.flowTokenAddress,
				nextAcc.address,
				tokensPerTransfer)
		},
		AddArguments: nil,
	}
	addKeysWorkload = &SimpleWorkLoad{
		Name: "TokenTransfer",
		Script: func(lg *ContLoadGenerator, workerID int, acc *flowAccount) ([]byte, error) {
			return AddKeyToAccountScript()
		},
		AddArguments: func(lg *ContLoadGenerator, workerID int, tx *flowsdk.Transaction) error {
			numberOfKeysToAdd := 40
			cadenceKeys := make([]cadence.Value, numberOfKeysToAdd)
			for i := 0; i < numberOfKeysToAdd; i++ {
				cadenceKeys[i] = bytesToCadenceArray(lg.serviceAccount.accountKey.Encode())
			}
			cadenceKeysArray := cadence.NewArray(cadenceKeys)
			return tx.AddArgument(cadenceKeysArray)
		},
	}
	computationHeavyWorkload = &SimpleWorkLoad{
		Name: "ComputationHeavy",
		Script: func(lg *ContLoadGenerator, workerID int, acc *flowAccount) ([]byte, error) {
			return ComputationHeavyScript(*lg.favContractAddress), nil
		},
		AddArguments: nil,
	}
	eventHeavyWorkload = &SimpleWorkLoad{
		Name: "EventHeavy",
		Script: func(lg *ContLoadGenerator, workerID int, acc *flowAccount) ([]byte, error) {
			return EventHeavyScript(*lg.favContractAddress), nil
		},
		AddArguments: nil,
	}
	ledgerHeavyWorkload = &SimpleWorkLoad{
		Name: "LedgerHeavy",
		Script: func(lg *ContLoadGenerator, workerID int, acc *flowAccount) ([]byte, error) {
			return LedgerHeavyScript(*lg.favContractAddress), nil
		},
		AddArguments: nil,
	}
)

// largeTxLoad is a load generator that sends large transactions
// the relative size is relative to the 95 percentile of transaction sizes seen on mainnet on 2021-12-13
func largeTxLoad(relativeSize float64) *SimpleWorkLoad {
	txSize95percentile := 3200.0 // 95th percentile of tx byte sizes from metrika on 2021-12-13

	txSize := int(uint(txSize95percentile * relativeSize))

	longString := strings.Repeat("a", txSize)

	return &SimpleWorkLoad{
		Name: "LedgerHeavy",
		Script: func(lg *ContLoadGenerator, workerID int, acc *flowAccount) ([]byte, error) {
			return []byte(
				fmt.Sprintf(`
					transaction{
						prepare(acct: AuthAccount){}
						execute{
							// %s
						}		
					}`, longString)), nil
		},
		AddArguments: nil,
	}
}
