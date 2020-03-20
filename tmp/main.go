package main

import (
	"fmt"
	"os"

	"github.com/dgraph-io/badger/v2"

	clusterstate "github.com/dapperlabs/flow-go/cluster/badger"
)

/*
>>>> col0 datadir:  /tmp/flow-integration643048995
>>>> col1 datadir:  /tmp/flow-integration241667878
genesis: b49d00863b0de72ccfdee131e25042cc472bc86445aead5ebdd663e9855a4ea3
*/

func main() {

	chainID := "c3205e42da165282eb10d4f2d820de75ca396644c6c4041268c23b904df5da0c"

	datadir := os.Args[1]

	fmt.Printf("datadir: '%s'\n", datadir)

	stat, err := os.Stat(datadir)
	if err != nil {
		panic(err)
	}
	fmt.Println(stat.Name(), stat.Size(), stat.IsDir())

	// create a database
	db, err := badger.Open(badger.DefaultOptions(datadir).WithLogger(nil))
	if err != nil {
		panic(err)
	}

	// try to read the default value
	err = db.View(func(tx *badger.Txn) error {
		item, err := tx.Get([]byte{1, 2, 3, 4})
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			fmt.Print("val: %x\n", val)
			return nil
		})
	})
	if err != nil {
		panic(err)
	}

	state, err := clusterstate.NewState(db, chainID)
	if err != nil {
		panic(err)
	}

	head, err := state.Final().Head()
	if err != nil {
		panic(err)
	}

	fmt.Println("cluster ID: ", chainID)
	fmt.Println("data dir: ", datadir)
	fmt.Println("head id: ", head.ID())
	fmt.Println("head height: ", head.Height)
	fmt.Println("had parent id: ", head.ParentID)
}
