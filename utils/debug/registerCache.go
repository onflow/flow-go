package debug

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"

	"github.com/onflow/flow-go/model/flow"
)

type RegisterCache interface {
	Get(owner, key string) (value []byte, found bool)
	Set(owner, key string, value []byte)
	Persist() error
}

func newRegisterKey(owner string, key string) string {
	return owner + "~" + key
}

type InMemoryRegisterCache struct {
	data map[string]flow.RegisterValue
}

var _ RegisterCache = &InMemoryRegisterCache{}

func NewInMemoryRegisterCache() *InMemoryRegisterCache {
	return &InMemoryRegisterCache{data: make(map[string]flow.RegisterValue)}

}
func (c *InMemoryRegisterCache) Get(owner, key string) ([]byte, bool) {
	v, found := c.data[newRegisterKey(owner, key)]
	return v, found
}

func (c *InMemoryRegisterCache) Set(owner, key string, value []byte) {
	c.data[newRegisterKey(owner, key)] = value
}

func (c *InMemoryRegisterCache) Persist() error {
	// No-op
	return nil
}

type FileRegisterCache struct {
	filePath string
	data     map[string]flow.RegisterEntry
}

var _ RegisterCache = &FileRegisterCache{}

func NewFileRegisterCache(filePath string) (*FileRegisterCache, error) {
	data := make(map[string]flow.RegisterEntry)

	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := bufio.NewReader(f)
	var s string
	for {
		s, err = r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if len(s) > 0 {
			var d flow.RegisterEntry
			if err := json.Unmarshal([]byte(s), &d); err != nil {
				return nil, err
			}

			owner, err := hex.DecodeString(d.Key.Owner)
			if err != nil {
				return nil, err
			}

			keyCopy, err := hex.DecodeString(d.Key.Key)
			if err != nil {
				return nil, err
			}

			data[newRegisterKey(string(owner), string(keyCopy))] = d
		}
	}

	return &FileRegisterCache{
		filePath: filePath,
		data:     data,
	}, nil
}

func (c *FileRegisterCache) Get(owner, key string) ([]byte, bool) {
	v, found := c.data[newRegisterKey(owner, key)]
	if !found {
		return nil, found
	}

	return v.Value, found
}

func (c *FileRegisterCache) Set(owner, key string, value []byte) {
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	ownerAddr := flow.BytesToAddress([]byte(owner))

	c.data[newRegisterKey(owner, key)] = flow.RegisterEntry{
		Key:   flow.NewRegisterID(ownerAddr, hex.EncodeToString([]byte(key))),
		Value: valueCopy,
	}
}

func (c *FileRegisterCache) Persist() error {
	f, err := os.Create(c.filePath)
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

		_, err = w.Write(fltV)
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

	return nil
}
