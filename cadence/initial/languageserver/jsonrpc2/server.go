/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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

func (server *Server) Call(method string, params interface{}) error {
	return server.conn.Call(context.Background(), method, params, nil)
}
