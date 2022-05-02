//package main
//
//import (
//	"bytes"
//	"fmt"
//	"io"
//	"log"
//
//	z "github.com/klauspost/compress/zstd"
//	csc "github.com/onflow/flow-go/cadence_script_compression"
//)
//
//const (
//	mainnetDir = "../../contracts/mainnet"
//)
//
//func main() {
//	contracts := csc.ReadContracts(mainnetDir)
//
//	sumOfRatios := float64(0)
//	for _, c := range contracts {
//		compData := &csc.CompressionComparison{
//			CompressedData:   make([]byte, 0),
//			UncompressedData: c.Data,
//		}
//
//		r := bytes.NewReader(compData.UncompressedData)
//		w := bytes.NewBuffer(compData.CompressedData)
//		err := Compress(r, w)
//		if err != nil {
//			panic(err)
//		}
//
//		compData.CompressedData = w.Bytes()
//
//		sumOfRatios = sumOfRatios + compData.CompressionRatio()
//		log.Println(fmt.Sprintf("Name: %s, Uncompressed: %d, Compressed: %d Ratio: %f", c.Name, compData.UnCompressedSize(), compData.CompressedSize(), compData.CompressionRatio()))
//	}
//
//	log.Println(fmt.Sprintf("Average compression Ratio: %f", sumOfRatios/float64(len(contracts))))
//}
//
//// Compress input to output.
//func Compress(in io.Reader, out io.Writer) error {
//	enc, err := z.NewWriter(out, []z.EOption{z.WithEncoderLevel(z.SpeedDefault)}...)
//	if err != nil {
//		return err
//	}
//	_, err = io.Copy(enc, in)
//	if err != nil {
//		enc.Close()
//		return err
//	}
//	return enc.Close()
//}
