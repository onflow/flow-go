package debug

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/rs/zerolog"
)

type registerCache interface {
	Get(owner, controller, key string) (value []byte, found bool)
	Set(owner, controller, key string, value []byte)
	Persist() error
}

type memRegisterCache struct {
	data map[string]flow.RegisterValue
}

func newMemRegisterCache() *memRegisterCache {
	return &memRegisterCache{data: make(map[string]flow.RegisterValue)}

}
func (c *memRegisterCache) Get(owner, controller, key string) ([]byte, bool) {
	v, found := c.data[owner+"~"+controller+"~"+key]
	return v, found
}

func (c *memRegisterCache) Set(owner, controller, key string, value []byte) {
	c.data[owner+"~"+controller+"~"+key] = value
}
func (c *memRegisterCache) Persist() error {
	// No-op
	return nil
}

type fileRegisterCache struct {
	filePath string
	data     map[string]flow.RegisterEntry
}

func newFileRegisterCache(filePath string) *fileRegisterCache {
	cache := &fileRegisterCache{filePath: filePath}
	data := make(map[string]flow.RegisterEntry)

	if _, err := os.Stat(filePath); err == nil {
		f, err := os.Open(filePath)
		if err != nil {
			fmt.Printf("error opening file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		r := bufio.NewReader(f)
		var s string
		for {
			s, err = r.ReadString('\n')
			if err != nil && err != io.EOF {
				break
			}
			if len(s) > 0 {
				var d flow.RegisterEntry
				if err := json.Unmarshal([]byte(s), &d); err != nil {
					panic(err)
				}
				owner, err := hex.DecodeString(d.Key.Owner)
				if err != nil {
					panic(err)
				}
				controller, err := hex.DecodeString(d.Key.Controller)
				if err != nil {
					panic(err)
				}
				keyCopy, err := hex.DecodeString(d.Key.Key)
				if err != nil {
					panic(err)
				}
				data[string(owner)+"~"+string(controller)+"~"+string(keyCopy)] = d
			}
			if err != nil {
				break
			}
		}
	}

	cache.data = data
	return cache
}

func (f *fileRegisterCache) Get(owner, controller, key string) ([]byte, bool) {
	v, found := f.data[owner+"~"+controller+"~"+key]
	if found {
		return v.Value, found
	}
	return nil, found
}

func (f *fileRegisterCache) Set(owner, controller, key string, value []byte) {
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)
	fmt.Println(hex.EncodeToString([]byte(owner)), hex.EncodeToString([]byte(key)), len(value))
	f.data[owner+"~"+controller+"~"+key] = flow.RegisterEntry{
		Key: flow.NewRegisterID(hex.EncodeToString([]byte(owner)),
			hex.EncodeToString([]byte(controller)),
			hex.EncodeToString([]byte(key))),
		Value: flow.RegisterValue(valueCopy),
	}
}

func (c *fileRegisterCache) Persist() error {
	f, err := os.OpenFile(c.filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, v := range c.data {
		fltV, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = w.WriteString(string(fltV))
		if err != nil {
			return err
		}
		err = w.WriteByte('\n')
		if err != nil {
			return err
		}
	}
	err = w.Flush()
	if err != nil {
		return err
	}
	err = f.Sync()
	if err != nil {
		return err
	}
	return nil
}

type checkpointBasedRegisterCache struct {
	ledgerPath string
	state      ledger.State
	ledger     ledger.Ledger
}

func newCheckpointBasedRegisterCache(ledgerPath string, targetstate string) (*checkpointBasedRegisterCache, error) {

	diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, &metrics.NoopCollector{}, ledgerPath, complete.DefaultCacheSize, pathfinder.PathByteSize, wal.SegmentSize)
	if err != nil {
		return nil, fmt.Errorf("cannot create WAL: %w", err)
	}
	defer func() {
		<-diskWal.Done()
	}()
	led, err := complete.NewLedger(diskWal, complete.DefaultCacheSize, &metrics.NoopCollector{}, zerolog.Nop(), 0)
	if err != nil {
		return nil, fmt.Errorf("cannot create ledger from write-a-head logs and checkpoints: %w", err)
	}

	var state ledger.State
	// if targetstate is empty take the latest one from checkpoint
	if len(targetstate) == 0 {
		state, err = led.MostRecentTouchedState()
		if err != nil {
			return nil, fmt.Errorf("failed to load the most recently touched state from ledger: %w", err)
		}
	} else {
		stateBytes, err := hex.DecodeString(targetstate)
		if err != nil {
			return nil, fmt.Errorf("failed to decode hex code of state: %w", err)
		}
		state, err = ledger.ToState(stateBytes)
		if err != nil {
			return nil, fmt.Errorf("cannot use the input state: %w", err)
		}
	}

	return &checkpointBasedRegisterCache{
		ledgerPath: ledgerPath,
		ledger:     led,
		state:      state,
	}, nil
}

func (f *checkpointBasedRegisterCache) Get(owner, controller, key string) ([]byte, bool) {
	k := ledger.NewKey([]ledger.KeyPart{
		ledger.NewKeyPart(0, []byte(owner)),
		ledger.NewKeyPart(1, []byte(controller)),
		ledger.NewKeyPart(2, []byte(key)),
	})
	query, err := ledger.NewQuery(f.state, []ledger.Key{k})
	if err != nil {
		panic(err)
	}
	values, err := f.ledger.Get(query)
	if err != nil {
		panic(err)
	}
	if len(values) != 1 {
		panic("ledger returned more than 1 value")
	}
	return values[0], len(values[0]) > 0
}

func (f *checkpointBasedRegisterCache) Set(owner, controller, key string, value []byte) {
	// no op
}

func (c *checkpointBasedRegisterCache) Persist() error {
	// no op
	return nil
}
