package main

import (
	"fmt"
	"log"

	csc "github.com/onflow/flow-go/cadence_script_compression"
)

func main()  {
	//goztsd.Compress( )
	mainnetContractsDir := "../contracts/mainnet"
	contracts := csc.ReadContracts(mainnetContractsDir)
	largest := &csc.Contract{Name: "", Size: 0, Data: nil}

	for _, c := range contracts {
		if c.Size > largest.Size {
			largest = c
		}
		log.Println(fmt.Sprintf("Name: %s, Size: %d", c.Name, c.Size))
	}

	log.Println("LARGEST: ", largest.Name, largest.Size)
}
