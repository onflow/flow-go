package uploader

import (
	"bufio"
	"fmt"
	"os"
	"path"

	"github.com/onflow/flow-go/engine/execution"
)

func NewFileUploader(dir string) *FileUploader {
	return &FileUploader{
		dir: dir,
	}
}

type FileUploader struct {
	dir string
}

func (f *FileUploader) Upload(computationResult *execution.ComputationResult) error {
	file, err := os.Create(path.Join(f.dir, fmt.Sprintf("%s.cbor", computationResult.ExecutableBlock.ID())))
	if err != nil {
		return fmt.Errorf("cannot create file for writing block data: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	return WriteComputationResultsTo(computationResult, writer)
}
