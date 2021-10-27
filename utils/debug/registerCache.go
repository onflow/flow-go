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
		w.WriteString(string(fltV))
		w.WriteByte('\n')
	}
	w.Flush()
	f.Sync()
	return nil
}
