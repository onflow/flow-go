package protocol

import "github.com/dapperlabs/flow-go/language/tools/language-server/jsonrpc2"

// Server implements a layer between the JSONPRC2 server (jsonrpc2 package) and
// the business logic of the language server (server package).
//
// It handles unmarshalling inputs to the appropriate parameter types.
type Server struct {
	Handler        Handler
	conn           Conn
	jsonrpc2Server *jsonrpc2.Server
}

// Conn represents the connection to the language server client. It allows
// the language server to push various types of messages to the client.
// https://microsoft.github.io/language-server-protocol/specifications/specification-3-14
type Conn interface {
	ShowMessage(params *ShowMessageParams)
	LogMessage(params *LogMessageParams)
	PublishDiagnostics(params *PublishDiagnosticsParams)
	RegisterCapability(params *RegistrationParams) error
}

type connection struct {
	jsonrpc2Server *jsonrpc2.Server
}

// ShowMessage displays a notification message to the client. It is visible to
// the user.
func (conn *connection) ShowMessage(params *ShowMessageParams) {
	conn.jsonrpc2Server.Notify("window/showMessage", params)
}

// LogMessage logs a message to the Cadence terminal in VS Code. It isn't
// visible to the user unless they go looking for it.
func (conn *connection) LogMessage(params *LogMessageParams) {
	conn.jsonrpc2Server.Notify("window/logMessage", params)
}

// PublishDiagnostics is used to report errors for a document, typically syntax
// or semantic errors in the code.
func (conn *connection) PublishDiagnostics(params *PublishDiagnosticsParams) {
	conn.jsonrpc2Server.Notify("textDocument/publishDiagnostics", params)
}

// RegisterCapability is used to dynamically inform the client that the server
// supports a particular API.
func (conn *connection) RegisterCapability(params *RegistrationParams) error {
	return conn.jsonrpc2Server.Call("client/registerCapability", params)
}

// Handler defines the subset of the Language Server Protocol we support.
type Handler interface {
	Initialize(conn Conn, params *InitializeParams) (*InitializeResult, error)
	DidChangeTextDocument(conn Conn, params *DidChangeTextDocumentParams) error
	Hover(conn Conn, params *TextDocumentPositionParams) (*Hover, error)
	Definition(conn Conn, params *TextDocumentPositionParams) (*Location, error)
	SignatureHelp(conn Conn, params *TextDocumentPositionParams) (*SignatureHelp, error)
	CodeLens(conn Conn, params *CodeLensParams) ([]*CodeLens, error)
	ExecuteCommand(conn Conn, params *ExecuteCommandParams) (interface{}, error)
	Shutdown(conn Conn) error
	Exit(conn Conn) error
}

func NewServer(handler Handler) *Server {
	jsonrpc2Server := jsonrpc2.NewServer()

	conn := &connection{
		jsonrpc2Server: jsonrpc2Server,
	}

	server := &Server{
		Handler:        handler,
		conn:           conn,
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

	jsonrpc2Server.Methods["textDocument/codeLens"] =
		server.handleCodeLens

	jsonrpc2Server.Methods["workspace/executeCommand"] =
		server.handleExecuteCommand

	jsonrpc2Server.Methods["shutdown"] =
		server.handleShutdown

	jsonrpc2Server.Methods["exit"] =
		server.handleExit

	return server
}

func (server *Server) Start() <-chan struct{} {
	return server.jsonrpc2Server.Start()
}
