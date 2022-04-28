package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
)

func main()  {
	// Open the file
	csvfile, err := os.Open("./contracts/mainnet/mainnet-contract-data.csv")
	if err != nil {
		panic(err)
	}

	// Parse the file
	r := csv.NewReader(csvfile)

	// Iterate through the records
	for {
		// Read each record from csv
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		fileName := getFileName(record[0], record[1])

		f, err := os.Create(fmt.Sprintf("./contracts/mainnet/%s", fileName))
		if err != nil {
			fmt.Println(err)
			return
		}

		_, _ = f.WriteString(record[2])
		f.Close()
	}
}

func getFileName(address, name string) string {
	return fmt.Sprintf("%s.%s.cdc", address, name)
}
