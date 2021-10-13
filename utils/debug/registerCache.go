package debug

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/onflow/flow-go/model/flow"
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
	return errors.New("MemRegisterCache doesn't implement a persist option, try using a fileRegisterCache")
}

// TODO optimize this to not be append only operation
type fileRegisterCache struct {
	filePath string
	data     map[string]flow.RegisterEntry
}

func newFileRegisterCache(filePath string) *fileRegisterCache {
	cache := &fileRegisterCache{filePath: filePath}
	data := make(map[string]flow.RegisterEntry)

	if _, err := os.Stat(filePath); os.IsExist(err) {
		f, err := os.Open(filePath)
		if err != nil {
			fmt.Printf("error opening file: %v\n", err)
			os.Exit(1)
		}
		r := bufio.NewReader(f)
		s, _, e := r.ReadLine()
		for e == nil {
			var d flow.RegisterEntry
			if err := json.Unmarshal(s, &d); err != nil {
				panic(err)
			}
			data[string(d.Key.Owner)+"~"+string(d.Key.Controller)+"~"+string(d.Key.Key)] = d
			s, _, e = r.ReadLine()
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
	f.data[owner+"~"+controller+"~"+key] = flow.RegisterEntry{
		Key:   flow.NewRegisterID(owner, controller, key),
		Value: flow.RegisterValue(value),
	}
}

func (c *fileRegisterCache) Persist() error {
	f, err := os.OpenFile(c.filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, v := range c.data {
		fltV, err := json.Marshal(v)
		if err != nil {
			return err
		}
		w.WriteString(string(fltV))
		w.WriteByte('\n')
	}
	return nil
}
