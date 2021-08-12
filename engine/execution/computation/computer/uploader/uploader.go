package uploader

import (
	"bufio"
	"fmt"
	"os"
	"path"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/flow-go/engine/execution"
)

type Uploader interface {
	Upload(computationResult *execution.ComputationResult) error
}

type AsyncUploader struct {
}

func (a *AsyncUploader) Upload(computationResult *execution.ComputationResult) error {
	return nil
}

type GCPBucketUploader struct {
}

func (u *GCPBucketUploader) Upload(computationResult *execution.ComputationResult) error {

	return nil
}

type FileUploader struct {
	dir string
}

func (f *FileUploader) Upload(computationResult *execution.ComputationResult) error {
	blockData := ComputationResultToBlockData(computationResult)

	file, err := os.Create(path.Join(f.dir, fmt.Sprintf("%s.cbor", computationResult.ExecutableBlock.ID())))
	if err != nil {
		return fmt.Errorf("cannot create file for writing block data: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	encoder := cbor.NewEncoder(writer)

	return encoder.Encode(blockData)
}
