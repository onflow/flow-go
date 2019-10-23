package main

import (
	"text/template"
)

// registryTemplate is the template used when generating registry code

var registryTemplate = template.Must(template.New("registry").Parse(registryTmplText))

// registryTmplText contains the template structure to be used
const registryTmplText = `package {{ .Package }}

import (
	"context"
	"fmt"

	gossip "github.com/dapperlabs/flow-go/network/gossip"
	proto "github.com/golang/protobuf/proto"
)

{{- range $reg := .Registries }}
type {{ $reg.InterfaceLong }}Registry struct {
	{{ $reg.InterfaceShort }} {{ $reg.InterfaceLong }}
}

// To make sure the class complies with the gossip.Registry interface
var _ gossip.Registry = (*{{ $reg.InterfaceLong }}Registry)(nil)

func New{{ $reg.InterfaceLong }}Registry({{ $reg.InterfaceShort }} {{ $reg.InterfaceLong }}) *{{ $reg.InterfaceLong }}Registry {
	return &{{ $reg.InterfaceLong }}Registry{
		{{ $reg.InterfaceShort }}: {{ $reg.InterfaceShort }},
	}
}

{{- range $reg.Methods }}

func ({{ $reg.InterfaceShort }}r *{{ $reg.InterfaceLong }}Registry) {{ .Name }}(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payload
	payload := &{{ .ParamType }}{}
	err := proto.Unmarshal(payloadByte, payload)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal payload: %v", err)
	}

	resp, respErr := {{ $reg.InterfaceShort }}r.{{ $reg.InterfaceShort }}.{{ .Name }}(ctx, payload)

	// Marshaling response
	respByte, err := proto.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("could not marshal response: %v", err)
	}

	return respByte, respErr
}
{{- end}}

func ({{ .InterfaceShort }}r *{{ .InterfaceLong }}Registry) MessageTypes() map[uint64]gossip.HandleFunc {
	return map[uint64]gossip.HandleFunc{
	 {{- range $Index, $Method := .Methods }}
		{{ $Index }}: {{ $reg.InterfaceShort }}r.{{ $Method.Name }},
		{{- end}}
	}
}

func ({{ .InterfaceShort }}r *{{ .InterfaceLong }}Registry) NameMapping() map[string]uint64{
	return map[string]uint64{
	 {{- range $Index, $Method := .Methods }}
		"{{ $Method.Name }}": {{ $Index }},
		{{- end}}
	}
}
{{- end}}`
