package websockets

import (
	"github.com/gorilla/websocket"
)

// We wrap gorilla's websocket connection with interface
// to be able to mock it in order to test the types dependent on it

type WebsocketConnection interface {
	ReadJSON(v interface{}) error
	WriteJSON(v interface{}) error
	Close() error
}

type GorillaWebsocketConnection struct {
	conn *websocket.Conn
}

func NewGorillaWebsocketConnection(conn *websocket.Conn) *GorillaWebsocketConnection {
	return &GorillaWebsocketConnection{
		conn: conn,
	}
}

var _ WebsocketConnection = (*GorillaWebsocketConnection)(nil)

func (m *GorillaWebsocketConnection) ReadJSON(v interface{}) error {
	return m.conn.ReadJSON(v)
}

func (m *GorillaWebsocketConnection) WriteJSON(v interface{}) error {
	return m.conn.WriteJSON(v)
}

func (m *GorillaWebsocketConnection) SetCloseHandler(handler func(code int, text string) error) {
	m.conn.SetCloseHandler(handler)
}

func (m *GorillaWebsocketConnection) Close() error {
	return m.conn.Close()
}
