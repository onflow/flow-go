package cadence_script_compression

import (
	"os"
	"path/filepath"
)


type Contract struct {
	Data []byte
	Name string
	Size int64
}

func ReadContracts(dirPath string) []*Contract {
	contracts := make([]*Contract, 0)

	files, _ := os.ReadDir(dirPath)
	path, _ := filepath.Abs(dirPath)
	for _, file := range files {
		realPath := filepath.Join(path, file.Name())
		info , err := file.Info()
		if err != nil {
			panic(err)
		}

		f, err := os.ReadFile(realPath)
		if err != nil {
			panic(err)
		}

		contracts = append(contracts, &Contract{
			Data: f,
			Name: info.Name(),
			Size: info.Size(),
		})
	}

	return contracts
}
