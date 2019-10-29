package main

// Collector is an example of a mock distributed storage system
// built using the collector gnode registry and gisp script
// It encompasses a step-by-step configuration of the gossip layer as well as marshalling and unmarshaling
// protobuf messages into and from bytes
import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang/protobuf/proto"

	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/dapperlabs/flow-go/network/gossip/examples/collector"
	"github.com/dapperlabs/flow-go/network/gossip/protocols"
	"github.com/dapperlabs/flow-go/proto/sdk/entities"
	"github.com/dapperlabs/flow-go/proto/services/collection"
)

func main() {
	key := flag.String("key", "", "specify key")
	flag.Parse()
	flag.Usage = func() {
		fmt.Printf("Usage: %v -key [key] [operation]\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Printf("Put example:\n\t %v -key test put\n", os.Args[0])
		fmt.Printf("Check example:\n\t %v -key test check\n", os.Args[0])
	}

	// printing the usage of the flags upon an error
	if len(flag.Args()) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	if *key == "" {
		fmt.Fprintf(os.Stderr, "[ERR]: key cannot be empty\n\n")
		flag.Usage()
		os.Exit(1)
	}

	operation := flag.Args()[0]

	switch operation {
	case "put":
		err := PutKey(*key)
		if err != nil {
			log.Fatal(err)
		}
	case "check":
		err := CheckKey(*key)
		if err != nil {
			log.Fatal(err)
		}
	}
}

// PutKey distributes a text to the servers whose address is specified in serverAddress
func PutKey(key string) error {
	serverAddress := []string{"127.0.0.1:50000", "127.0.0.1:50001", "127.0.0.1:50002"}
	colReg := collection.NewCollectServiceServerRegistry(collector.NewCollector())
	config := gossip.NewNodeConfig(colReg, "127.0.0.1:50004", serverAddress, 2, 10)
	node := gossip.NewNode(config)
	protocol := protocols.NewGServer(node)
	node.SetProtocol(protocol)

	sp, err := protocols.NewGServer(node)
	if err != nil {
		log.Fatalf("could not start network server: %v", err)
	}
	node.SetProtocol(sp)

	subRequest, err := GenerateSubmitTransactionRequest(key)
	if err != nil {
		return err
	}

	_, err = node.SyncGossip(context.Background(), subRequest, serverAddress, "SubmitTransaction")
	if err != nil {
		return fmt.Errorf("could not reach some servers: %v", err)
	}

	return nil
}

// CheckKey checks whether the key exists in the distributed storage
func CheckKey(key string) error {
	storageAddrs := []string{"127.0.0.1:50000", "127.0.0.1:50001", "127.0.0.1:50002"}

	colReg := collection.NewCollectServiceServerRegistry(collector.NewCollector())
	config := gossip.NewNodeConfig(colReg, "127.0.0.1:50004", storageAddrs, 2, 10)
	node := gossip.NewNode(config)
	sp, err := protocols.NewGServer(node)
	if err != nil {
		log.Fatalf("could not start network server: %v", err)
	}
	node.SetProtocol(sp)
	getRequest, err := GenerateGetTransactionRequest(key)
	if err != nil {
		return err
	}

	responses, err := node.SyncGossip(context.Background(), getRequest, storageAddrs, "GetTransaction")
	// check responses if they contain the specified key
	for _, resp := range responses {
		getResp, err := ExtractGetResp(resp.GetResponseByte())
		if err != nil {
			log.Printf("Error in unmarshalling response: %v\n", err)
			continue
		}
		// if at least one copy was found of the specified key, it reports it has been found and terminates
		if getResp != nil && getResp.GetTransaction() != nil {
			fmt.Println("=== Exists ===")
			fmt.Printf("Key: %v\n", string(getResp.GetTransaction().Script))
			return nil
		}
	}

	return fmt.Errorf("key: %v was not found", key)
}

// A Good Marshalling example

// GenerateSubmitTransactionRequest creates a SubmitTransactionRequest protobuf message, fills it with the text, and
// marshals it into bytes
func GenerateSubmitTransactionRequest(text string) ([]byte, error) {
	transaction := &entities.Transaction{Script: []byte(text)}
	byteRequest, err := proto.Marshal(&collection.SubmitTransactionRequest{Transaction: transaction})
	if err != nil {
		return nil, fmt.Errorf("could not marshal message: %v", err)
	}
	return byteRequest, nil
}

// A Good Marshalling example

// GenerateGetTransactionRequest creates a TransactionRequest protobuf message, fills it with the text, and
// marshals it into bytes
func GenerateGetTransactionRequest(text string) ([]byte, error) {
	request := &collection.GetTransactionRequest{Hash: []byte(text)}
	byteRequest, err := proto.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("could not marshal message: %v", err)
	}
	return byteRequest, nil
}

// A Good Unmarshalling example

// ExtractGetResp decodes a byte response into its original type and returns it
func ExtractGetResp(byteResponse []byte) (*collection.GetTransactionResponse, error) {
	reply := &collection.GetTransactionResponse{}
	if err := proto.Unmarshal(byteResponse, reply); err != nil {
		return nil, fmt.Errorf("could not unmarshal response: %v", err)
	}
	return reply, nil
}
