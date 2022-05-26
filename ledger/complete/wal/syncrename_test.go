package wal

import (
	"bufio"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func Test_RenameHappensAfterClosing(t *testing.T) {

	unittest.RunWithTempDir(t, func(dir string) {

		filename := "target.filename"
		tmpName := "tmp.filename"
		fullFileName := path.Join(dir, filename)
		fullTmpName := path.Join(dir, tmpName)

		require.NoFileExists(t, fullFileName)
		require.NoFileExists(t, fullTmpName)

		file, err := os.Create(fullTmpName)
		require.NoError(t, err)

		writer := bufio.NewWriter(file)

		syncer := &SyncOnCloseRenameFile{
			file:       file,
			targetName: fullFileName,
			Writer:     writer,
		}

		sampleBytes := []byte{2, 1, 3, 7}
		_, err = syncer.Write(sampleBytes)
		require.NoError(t, err)

		require.FileExists(t, fullTmpName)
		require.NoFileExists(t, fullFileName)

		err = syncer.Close()
		require.NoError(t, err)

		require.NoFileExists(t, fullTmpName)
		require.FileExists(t, fullFileName)

		readBytes, err := os.ReadFile(fullFileName)
		require.NoError(t, err)

		require.Equal(t, readBytes, sampleBytes)
	})
}
