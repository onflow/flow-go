package protocol

import "github.com/dapperlabs/flow-go/language/tools/language-server/jsonrpc2"

type Server struct {
	Handler        Handler
	connection     Connection
	jsonrpc2Server *jsonrpc2.Server
}

type Connection interface {
	ShowMessage(params *ShowMessageParams)
	LogMessage(params *LogMessageParams)
	PublishDiagnostics(params *PublishDiagnosticsParams)
}

type connection struct {
	jsonrpc2Server *jsonrpc2.Server
}

func (conn *connection) ShowMessage(params *ShowMessageParams) {
	conn.jsonrpc2Server.Notify("window/showMessage", params)
}

func (conn *connection) LogMessage(params *LogMessageParams) {
	conn.jsonrpc2Server.Notify("window/logMessage", params)
}

func (conn *connection) PublishDiagnostics(params *PublishDiagnosticsParams) {
	conn.jsonrpc2Server.Notify("textDocument/publishDiagnostics", params)
}

type Handler interface {
	Initialize(connection Connection, params *InitializeParams) (*InitializeResult, error)
	DidChangeTextDocument(connection Connection, params *DidChangeTextDocumentParams) error
	Hover(connection Connection, params *TextDocumentPositionParams) (*Hover, error)
	Definition(connection Connection, params *TextDocumentPositionParams) (*Location, error)
	SignatureHelp(connection Connection, params *TextDocumentPositionParams) (*SignatureHelp, error)
	Shutdown(connection Connection) error
	Exit(connection Connection) error
}

func NewServer(handler Handler) *Server {
	jsonrpc2Server := jsonrpc2.NewServer()

	conn := &connection{
		jsonrpc2Server: jsonrpc2Server,
	}

	server := &Server{
		Handler:        handler,
		connection:     conn,
		jsonrpc2Server: jsonrpc2Server,
	}

	jsonrpc2Server.Methods["initialize"] =
		server.handleInitialize

	jsonrpc2Server.Methods["textDocument/didChange"] =
		server.handleDidChangeTextDocument

	jsonrpc2Server.Methods["textDocument/hover"] =
		server.handleHover

	jsonrpc2Server.Methods["textDocument/definition"] =
		server.handleDefinition

	jsonrpc2Server.Methods["textDocument/signatureHelp"] =
		server.handleSignatureHelp

	jsonrpc2Server.Methods["shutdown"] =
		server.handleShutdown

	jsonrpc2Server.Methods["exit"] =
		server.handleExit

	return server
}

func (server *Server) Start() <-chan struct{} {
	return server.jsonrpc2Server.Start()
}
