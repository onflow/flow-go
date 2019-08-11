package protocol

import "encoding/json"

func (server *Server) handleInitialize(req *json.RawMessage) (interface{}, error) {
	var params InitializeParams
	if err := json.Unmarshal(*req, &params); err != nil {
		return nil, err
	}

	return server.Handler.Initialize(server.connection, &params)
}

func (server *Server) handleDidChangeTextDocument(req *json.RawMessage) (interface{}, error) {
	var params DidChangeTextDocumentParams
	if err := json.Unmarshal(*req, &params); err != nil {
		return nil, err
	}

	err := server.Handler.DidChangeTextDocument(server.connection, &params)
	return nil, err
}

func (server *Server) handleShutdown(req *json.RawMessage) (interface{}, error) {
	err := server.Handler.Shutdown(server.connection)
	return nil, err
}

func (server *Server) handleExit(req *json.RawMessage) (interface{}, error) {
	err := server.Handler.Exit(server.connection)
	return nil, err
}
