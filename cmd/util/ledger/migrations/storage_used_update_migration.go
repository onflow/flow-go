package migrations

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"runtime"
	"sync"
	"time"

	fvm "github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/model/flow"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type accountPayloadSize struct {
	Address     string
	StorageUsed uint64
}

type indexedPayload struct {
	Index   int
	Payload ledger.Payload
}

type accountStorageUsedPayload struct {
	Address string
	Index   int
}

type StorageUsedUpdateMigration struct {
	Log       zerolog.Logger
	OutputDir string
}

func (m *StorageUsedUpdateMigration) filename() string {
	return path.Join(m.OutputDir, fmt.Sprintf("storage_used_update_%d.csv", int32(time.Now().Unix())))
}

// iterates through registers keeping a map of register sizes
// after it has reached the end it add storage used and storage capacity for each address
func (m *StorageUsedUpdateMigration) Migrate(payload []ledger.Payload) ([]ledger.Payload, error) {
	fn := m.filename()
	m.Log.Info().Msgf("Running Storage Used update. Saving output to %s.", fn)

	f, err := os.Create(fn)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = f.Close()
		if err != nil {
			panic(err)
		}
	}()

	writer := bufio.NewWriter(f)
	defer func() {
		err = writer.Flush()
		if err != nil {
			panic(err)
		}
	}()

	workerCount := runtime.NumCPU()

	storageUsed := make(map[string]uint64)
	storageUsedChan := make(chan accountPayloadSize, workerCount)
	payloadChan := make(chan indexedPayload)
	storageUsedPayloadChan := make(chan accountStorageUsedPayload, workerCount)
	storageUsedPayload := make(map[string]int)

	inputWG := &sync.WaitGroup{}
	outputWG := &sync.WaitGroup{}

	outputWG.Add(1)
	go func() {
		for payloadSize := range storageUsedChan {
			if _, ok := storageUsed[payloadSize.Address]; !ok {
				storageUsed[payloadSize.Address] = 0
			}
			storageUsed[payloadSize.Address] = storageUsed[payloadSize.Address] + payloadSize.StorageUsed
		}
		outputWG.Done()
	}()

	outputWG.Add(1)
	go func() {
		for su := range storageUsedPayloadChan {
			if _, ok := storageUsedPayload[su.Address]; ok {
				m.Log.Error().
					Str("address", flow.BytesToAddress([]byte(su.Address)).Hex()).
					Msg("Already found a storage used payload for this address. Is this a duplicate?")
			}
			storageUsedPayload[su.Address] = su.Index
		}
		outputWG.Done()
	}()

	for i := 0; i < workerCount; i++ {
		inputWG.Add(1)
		go func() {
			for p := range payloadChan {
				var id flow.RegisterID
				id, err = keyToRegisterID(p.Payload.Key)
				if err != nil {
					log.Error().Err(err).Msg("error converting key to register ID")
				}
				if len([]byte(id.Owner)) != flow.AddressLength {
					// not an address
					continue
				}
				if id.Key == fvm.KeyStorageUsed {
					storageUsedPayloadChan <- accountStorageUsedPayload{
						Address: id.Owner,
						Index:   p.Index,
					}
				}
				storageUsedChan <- accountPayloadSize{
					Address:     id.Owner,
					StorageUsed: uint64(registerSize(id, p.Payload)),
				}
			}
			inputWG.Done()
		}()
	}

	for i, p := range payload {
		payloadChan <- indexedPayload{
			Index:   i,
			Payload: p,
		}
	}

	close(payloadChan)
	inputWG.Wait()
	close(storageUsedChan)
	close(storageUsedPayloadChan)
	outputWG.Wait()

	if err != nil {
		return nil, err
	}

	if len(storageUsedPayload) != len(storageUsed) {
		errStr := "number off accounts and number of storage_used registers don't match"
		log.Error().Int("accounts", len(storageUsed)).Int("storageUsedPayloads", len(storageUsedPayload)).Msg(errStr)
		return nil, fmt.Errorf(errStr)
	}

	var change int64
	storageIncreaseCount := 0
	storageDecreaseCount := 0
	storageNoChangeCount := 0

	for a, pIndex := range storageUsedPayload {
		p := payload[pIndex]
		used, ok := storageUsed[a]
		if !ok {
			errStr := "address has a storage used payload but is not using any storage"
			log.Error().Str("address", flow.BytesToAddress([]byte(a)).Hex()).Msg(errStr)
			return nil, fmt.Errorf(errStr)
		}

		id, err := keyToRegisterID(p.Key)
		if err != nil {
			log.Error().Err(err).Msg("error converting key to register ID")
			return nil, err
		}
		if id.Key != fvm.KeyStorageUsed {
			return nil, fmt.Errorf("this is not a storage used register")
		}

		oldUsed, _, err := utils.ReadUint64(p.Value)
		if err != nil {
			errStr := "cannot decode storage used by address"
			log.Error().
				Str("address", flow.BytesToAddress([]byte(a)).Hex()).
				Hex("storageUsed", p.Value).
				Hex("storageUsedKey", p.Key.CanonicalForm()).
				Err(err).
				Msg(errStr)
			return nil, fmt.Errorf(errStr)
		}

		if oldUsed > used {
			storageDecreaseCount += 1
			change = -int64(oldUsed - used)
		} else if oldUsed == used {
			storageNoChangeCount += 1
			change = 0
		} else {
			storageIncreaseCount += 1
			change = int64(oldUsed - used)
		}
		_, err = writer.WriteString(fmt.Sprintf("%s,%d,%d,%d\n", flow.BytesToAddress([]byte(a)).Hex(), oldUsed, used, change))
		if err != nil {
			return nil, err
		}

		payload[pIndex].Value = utils.Uint64ToBinary(used)
	}

	m.Log.Info().
		Int("accountsStorageIncreased", storageIncreaseCount).
		Int("accountsStorageDecreased", storageDecreaseCount).
		Int("accountsStorageNoChangeCount", storageNoChangeCount).
		Msg("Storage used update complete.")

	return payload, nil
}
