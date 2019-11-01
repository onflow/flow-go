package jsonrpc2

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sourcegraph/jsonrpc2"
)

type handler struct {
	server *Server
}

func (handler *handler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	method, ok := handler.server.Methods[req.Method]

	if !ok {
		if req.Notif {
			return
		}

		errResponse := &jsonrpc2.Error{
			Code:    jsonrpc2.CodeMethodNotFound,
			Message: fmt.Sprintf("method %q not found", req.Method),
		}
		err := conn.ReplyWithError(ctx, req.ID, errResponse)
		if err != nil {
			panic(err)
		}

		return
	}

	result, err := method(req.Params)

	if req.Notif {
		return
	}

	if err != nil {
		errResponse := &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInternalError,
			Message: err.Error(),
		}
		err = conn.ReplyWithError(ctx, req.ID, errResponse)
	} else {
		err = conn.Reply(ctx, req.ID, result)
	}

	if err != nil {
		panic(err)
	}
}

type Method func(*json.RawMessage) (interface{}, error)

type Server struct {
	Methods map[string]Method
	conn    *jsonrpc2.Conn
}

func NewServer() *Server {
	return &Server{
		Methods: map[string]Method{},
	}
}

func (server *Server) Start() <-chan struct{} {
	stream := jsonrpc2.NewBufferedStream(stdrwc{}, jsonrpc2.VSCodeObjectCodec{})
	server.conn = jsonrpc2.NewConn(context.Background(), stream, &handler{server})
	return server.conn.DisconnectNotify()
}

func (server *Server) Notify(method string, params interface{}) {
	err := server.conn.Notify(context.Background(), method, params)
	if err != nil {
		panic(err)
	}
}
