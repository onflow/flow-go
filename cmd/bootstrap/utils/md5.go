package utils

// The google storage API only provides md5 and crc32 hence overriding the linter flag for md5
// #nosec
import (
	"crypto/md5"
	"io"
	"os"
)

func CalcMd5(outpath string) []byte {
	f, err := os.Open(outpath)
	if err != nil {
		return nil
	}
	defer f.Close()

	// #nosec
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil
	}

	return h.Sum(nil)
}
