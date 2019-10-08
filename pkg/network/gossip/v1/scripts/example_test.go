package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

func Example_generateCodeFromFile() {
	// specify the file you wish to generate a registry for
	filepath := "./code-examples/verify.pb.go"

	// execute the generateCodeFromFile function with the desired file
	generetedCode, err := generateCodeFromFile(filepath)
	if err != nil {
		log.Printf("could not generate code from given file")
		return
	}

	// save generated code to file
	tmpfile, err := ioutil.TempFile("", "*_registry.gen.go")
	tmpfile.WriteString(generetedCode)

	// You can also print it to standard out
	fmt.Print(generetedCode)
	// Output: package verify
	//
	// import (
	// 	"context"
	// 	"fmt"
	//
	// 	gnode "github.com/dapperlabs/flow-go/pkg/network/gossip/v1"
	// 	proto "github.com/golang/protobuf/proto"
	// )
	//
	// type VerifyServiceServerRegistry struct {
	// 	vss VerifyServiceServer
	// }
	//
	// // To make sure the class complies with the gnode.Registry interface
	// var _ gnode.Registry = (*VerifyServiceServerRegistry)(nil)
	//
	// func NewVerifyServiceServerRegistry(vss VerifyServiceServer) *VerifyServiceServerRegistry {
	// 	return &VerifyServiceServerRegistry{
	// 		vss: vss,
	// 	}
	// }
	//
	// func (vssr *VerifyServiceServerRegistry) Ping(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// 	// Unmarshaling payload
	// 	payload := &PingRequest{}
	// 	err := proto.Unmarshal(payloadByte, payload)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	// 	}
	//
	// 	resp, respErr := vssr.vss.Ping(ctx, payload)
	//
	// 	// Marshaling response
	// 	respByte, err := proto.Marshal(resp)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("could not marshal response: %v", err)
	// 	}
	//
	// 	return respByte, respErr
	// }
	//
	// func (vssr *VerifyServiceServerRegistry) SubmitExecutionReceipt(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// 	// Unmarshaling payload
	// 	payload := &SubmitExecutionReceiptRequest{}
	// 	err := proto.Unmarshal(payloadByte, payload)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	// 	}
	//
	// 	resp, respErr := vssr.vss.SubmitExecutionReceipt(ctx, payload)
	//
	// 	// Marshaling response
	// 	respByte, err := proto.Marshal(resp)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("could not marshal response: %v", err)
	// 	}
	//
	// 	return respByte, respErr
	// }
	//
	// func (vssr *VerifyServiceServerRegistry) MessageTypes() map[string]gnode.HandleFunc {
	// 	return map[string]gnode.HandleFunc{
	// 		"Ping":                   vssr.Ping,
	// 		"SubmitExecutionReceipt": vssr.SubmitExecutionReceipt,
	// 	}
	// }
}

func Example_parseCode_fromFile() {
	// Open code file
	file, err := os.Open("./code-examples/verify.pb.go")
	if err != nil {
		log.Printf("could not open example code: %v", err)
		return
	}

	// Pass the opened file to the parser
	// The parser will extract needed information from the code (such as package
	// name, interface name,...) and provide it in a simple structure called
	// fileInfo, which will be passed to the template in order to generate the
	// desired code.
	fileInfo, err := parseCode(file)
	if err != nil {
		log.Printf("could not parse code: %v", err)
		return
	}

	fmt.Println(fileInfo)
	// Output: &{verify [{VerifyServiceServer vss [{Ping PingRequest} {SubmitExecutionReceipt SubmitExecutionReceiptRequest}]}]}

	// You can then generate the code from the given fileInfo using the
	// generateRegistry function
}

func Example_parseCode_fromRegistry() {
	file, err := os.Open("./code-examples/verify.pb.go")
	if err != nil {
		log.Printf("could not open example code: %v", err)
		return
	}

	fileInfo, err := parseCode(file)
	if err != nil {
		log.Printf("could not parse code: %v", err)
		return
	}

	// Pass the fileInfo optained from reading the desired file into
	// generateRegistry. It will execute the template with the given info, and also
	// format the go code (similar to gofmt).
	generatedCode, err := generateRegistry(fileInfo)
	if err != nil {
		log.Printf("could not generate registry from file info: %v", err)
		return

	}

	fmt.Println(generatedCode)
	// Output: package verify
	//
	// import (
	// 	"context"
	// 	"fmt"
	//
	// 	gnode "github.com/dapperlabs/flow-go/pkg/network/gossip/v1"
	// 	proto "github.com/golang/protobuf/proto"
	// )
	//
	// type VerifyServiceServerRegistry struct {
	// 	vss VerifyServiceServer
	// }
	//
	// // To make sure the class complies with the gnode.Registry interface
	// var _ gnode.Registry = (*VerifyServiceServerRegistry)(nil)
	//
	// func NewVerifyServiceServerRegistry(vss VerifyServiceServer) *VerifyServiceServerRegistry {
	// 	return &VerifyServiceServerRegistry{
	// 		vss: vss,
	// 	}
	// }
	//
	// func (vssr *VerifyServiceServerRegistry) Ping(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// 	// Unmarshaling payload
	// 	payload := &PingRequest{}
	// 	err := proto.Unmarshal(payloadByte, payload)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	// 	}
	//
	// 	resp, respErr := vssr.vss.Ping(ctx, payload)
	//
	// 	// Marshaling response
	// 	respByte, err := proto.Marshal(resp)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("could not marshal response: %v", err)
	// 	}
	//
	// 	return respByte, respErr
	// }
	//
	// func (vssr *VerifyServiceServerRegistry) SubmitExecutionReceipt(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// 	// Unmarshaling payload
	// 	payload := &SubmitExecutionReceiptRequest{}
	// 	err := proto.Unmarshal(payloadByte, payload)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	// 	}
	//
	// 	resp, respErr := vssr.vss.SubmitExecutionReceipt(ctx, payload)
	//
	// 	// Marshaling response
	// 	respByte, err := proto.Marshal(resp)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("could not marshal response: %v", err)
	// 	}
	//
	// 	return respByte, respErr
	// }
	//
	// func (vssr *VerifyServiceServerRegistry) MessageTypes() map[string]gnode.HandleFunc {
	// 	return map[string]gnode.HandleFunc{
	// 		"Ping":                   vssr.Ping,
	// 		"SubmitExecutionReceipt": vssr.SubmitExecutionReceipt,
	// 	}
	// }
	//
}
