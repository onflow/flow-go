package debug

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/onflow/flow-go/model/flow"
)

type RegisterCache interface {
	Get(owner, key string) (value []byte, found bool)
	Set(owner, key string, value []byte)
	Persist() error
}

type InMemoryRegisterCache struct {
	data map[string]flow.RegisterValue
}

var _ RegisterCache = &InMemoryRegisterCache{}

func NewInMemoryRegisterCache() *InMemoryRegisterCache {
	return &InMemoryRegisterCache{data: make(map[string]flow.RegisterValue)}

}
func (c *InMemoryRegisterCache) Get(owner, key string) ([]byte, bool) {
	v, found := c.data[owner+"~"+key]
	return v, found
}

func (c *InMemoryRegisterCache) Set(owner, key string, value []byte) {
	c.data[owner+"~"+key] = value
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

func NewFileRegisterCache(filePath string) *FileRegisterCache {
	cache := &FileRegisterCache{filePath: filePath}
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
				keyCopy, err := hex.DecodeString(d.Key.Key)
				if err != nil {
					panic(err)
				}
				data[string(owner)+"~"+string(keyCopy)] = d
			}
			if err != nil {
				break
			}
		}
	}

	cache.data = data
	return cache
}

func (f *FileRegisterCache) Get(owner, key string) ([]byte, bool) {
	v, found := f.data[owner+"~"+key]
	if found {
		return v.Value, found
	}
	return nil, found
}

func (f *FileRegisterCache) Set(owner, key string, value []byte) {
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)
	ownerAddr := flow.BytesToAddress([]byte(owner))
	fmt.Println(ownerAddr.Hex(), hex.EncodeToString([]byte(key)), len(value))
	f.data[owner+"~"+key] = flow.RegisterEntry{
		Key:   flow.NewRegisterID(ownerAddr, hex.EncodeToString([]byte(key))),
		Value: flow.RegisterValue(valueCopy),
	}
}

func (c *FileRegisterCache) Persist() error {
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
	return nil
}
