package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"
)

// testing parseCode function with multiple code snippets
func TestParseCodeWithStrings(t *testing.T) {
	tt := []struct {
		code string
		info *fileInfo
		err  error
	}{
		{
			code: `package home

			type TVRemoteServer interface {
					TurnOn(context.Context, *Void) (*Void, error)
					TurnOff(context.Context, *Void) (*Void, error)
			}`,
			info: &fileInfo{
				Package: "home",
				Registries: []registryInfo{
					{
						InterfaceLong:  "TVRemoteServer",
						InterfaceShort: "tvrs",
						Methods: []method{
							{Name: "TurnOn", ParamType: "Void"},
							{Name: "TurnOff", ParamType: "Void"},
						},
					},
				},
			},
			err: nil,
		},
		{ //Lowercase interface name
			code: `package home

			type alllowercaseinterfaceserver interface {
					TurnOn(context.Context, *Void) (*Void, error)
					TurnOff(context.Context, *Void) (*Void, error)
			}`,
			info: nil,
			err:  fmt.Errorf("non nil"),
		},
		{
			code: `package home

			// Interface name does not end with server
			type TVServerRemote interface {
					TurnOn(context.Context, *Void) (*Void, error)
					TurnOff(context.Context, *Void) (*Void, error)
			}`,
			info: nil,
			err:  errors.New("not nil"),
		},
		{
			code: `
			// Improper code
			TVServerRemote(
					TurnOn(context.Context, *Void) (*Void, error)
					TurnOff
			)`,
			info: nil,
			err:  errors.New("not nil"),
		},
		{
			code: "", //empty code
			info: nil,
			err:  errors.New("not nil"),
		},
	}

	for _, tc := range tt {
		codeBytes := []byte(tc.code)

		info, err := parseCode(bytes.NewReader(codeBytes))
		if info == nil && tc.info != nil {
			t.Errorf("info returned was nil but expected info was non nil")
		}

		if info != nil && tc.info == nil {
			t.Errorf("info returned was non nil but expected info was nil")
		}

		if tc.err == nil && err != nil {
			fmt.Println(tc.code)
			t.Errorf("function returned non nil error, but nil was expected: %v", err)
			continue
		}

		if tc.err != nil && err == nil {
			t.Errorf("function returned nil but an error was expected")
			continue
		}

		if info == nil && tc.info == nil {
			continue
		}

		if !reflect.DeepEqual(*info, *tc.info) {
			t.Errorf("constructed registryInfo does not match expected registry info:\n\tconsturcted: %v,\n\texpected: %v", *info, tc.info)
		}
	}
}

//Testing ParseCode with Reader inputs
func TestParseCodeWithReaders(t *testing.T) {
	tt := []struct {
		reader io.Reader
		info   *fileInfo
		err    error
	}{
		{
			reader: bytes.NewReader([]byte(`package home

			type TVRemoteServer interface {
					TurnOn(context.Context, *Void) (*Void, error)
					TurnOff(context.Context, *Void) (*Void, error)
			}`)),
			info: &fileInfo{
				Package: "home",
				Registries: []registryInfo{
					{
						InterfaceLong:  "TVRemoteServer",
						InterfaceShort: "tvrs",
						Methods: []method{
							{Name: "TurnOn", ParamType: "Void"},
							{Name: "TurnOff", ParamType: "Void"},
						},
					},
				},
			},
			err: nil,
		},
		{
			reader: FakeReader{},
			info:   nil,
			err:    errors.New("not nil"),
		},
	}

	for _, tc := range tt {
		info, err := parseCode(tc.reader)

		if tc.err == nil && err != nil {
			t.Errorf("function returned non nil error, but nil was expected: %v", err)
			continue
		}

		if tc.err != nil && err == nil {
			t.Errorf("function returned nil but an error was expected")
			continue
		}

		if info == nil && tc.info == nil {
			continue
		}

		if !reflect.DeepEqual(*info, *tc.info) {
			t.Errorf("constructed registryInfo does not match expected registry info:\n\tconsturcted: %v,\n\texpected: %v\n", *info, tc.info)
		}
	}
}

//Fake reader which implements io.Reader and returns an error when reading in order to check whether the function
//handles cases where the Reader throws an error
type FakeReader struct {
}

func (f FakeReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	return 0, errors.New("This is an error")
}

// testing generateRegistry function with a given registryInfo
func TestGenerateRegistry(t *testing.T) {
	tt := []struct {
		info    *fileInfo
		genCode string
		err     error
	}{
		{
			info: &fileInfo{
				Package: "home",
				Registries: []registryInfo{
					{
						InterfaceLong:  "TVRemoteServer",
						InterfaceShort: "tvrs",
						Methods: []method{
							{Name: "TurnOn", ParamType: "Void"},
							{Name: "TurnOff", ParamType: "Void"},
						},
					},
				},
			},
			genCode: `package home

import (
	"context"
	"fmt"

	gossip "github.com/dapperlabs/flow-go/network/gossip"
	proto "github.com/golang/protobuf/proto"
)

type TVRemoteServerRegistry struct {
	tvrs TVRemoteServer
}

// To make sure the class complies with the gossip.Registry interface
var _ gossip.Registry = (*TVRemoteServerRegistry)(nil)

func NewTVRemoteServerRegistry(tvrs TVRemoteServer) *TVRemoteServerRegistry {
	return &TVRemoteServerRegistry{
		tvrs: tvrs,
	}
}

func (tvrsr *TVRemoteServerRegistry) TurnOn(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &Void{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := tvrsr.tvrs.TurnOn(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (tvrsr *TVRemoteServerRegistry) TurnOff(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &Void{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := tvrsr.tvrs.TurnOff(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}

func (tvrsr *TVRemoteServerRegistry) MessageTypes() map[uint64]gossip.HandleFunc {
	return map[uint64]gossip.HandleFunc{
		0: tvrsr.TurnOn,
		1: tvrsr.TurnOff,
	}
}

func (tvrsr *TVRemoteServerRegistry) NameMapping() map[string]uint64 {
	return map[string]uint64{
		"TurnOn":  0,
		"TurnOff": 1,
	}
}
`,
			err: nil,
		},
		{
			info:    nil, //nil info
			genCode: "",
			err:     fmt.Errorf("not nil"),
		},
		{
			info: &fileInfo{ //invalid info which results in invalid code
				Package: "",
				Registries: []registryInfo{
					{
						InterfaceLong:  "",
						InterfaceShort: "",
						Methods: []method{
							{Name: "This is invalid{ { {", ParamType: "("},
						},
					},
				},
			},
			genCode: "",
			err:     fmt.Errorf("non nil"),
		},
	}

	for _, tc := range tt {
		genCode, err := generateRegistry(tc.info)
		if tc.err == nil && err != nil {
			t.Errorf("function expected to return nil, but returned a non nil error: %v", err)
			continue
		}

		if tc.err != nil && err == nil {
			t.Errorf("function expection to return an error, but nil was returned")
		}

		if tc.err != nil && err != nil {
			continue
		}

		if genCode != tc.genCode {
			fmt.Println(genCode)
			t.Error("generated code does not match expected code")
			fmt.Println(tc.genCode)
		}
	}

}
