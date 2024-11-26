package websockets

import (
	"time"

	"github.com/gorilla/websocket"
)

type WebsocketConnection interface {
	ReadJSON(v interface{}) error
	WriteJSON(v interface{}) error
	WriteMessage(int, []byte) error
	Close() error
	SetReadDeadline(time.Time) error
	SetWriteDeadline(time.Time) error
	SetPongHandler(func(string) error)
}

type WebsocketConnectionImpl struct {
	conn *websocket.Conn
}

func NewWebsocketConnection(conn *websocket.Conn) *WebsocketConnectionImpl {
	return &WebsocketConnectionImpl{
		conn: conn,
	}
}

var _ WebsocketConnection = (*WebsocketConnectionImpl)(nil)

func (c *WebsocketConnectionImpl) ReadJSON(v interface{}) error {
	return c.conn.ReadJSON(v)
}

func (c *WebsocketConnectionImpl) WriteJSON(v interface{}) error {
	return c.conn.WriteJSON(v)
}

func (c *WebsocketConnectionImpl) WriteMessage(messageType int, data []byte) error {
	return c.conn.WriteMessage(messageType, data)
}

func (c *WebsocketConnectionImpl) Close() error {
	return c.conn.Close()
}

func (c *WebsocketConnectionImpl) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *WebsocketConnectionImpl) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

func (c *WebsocketConnectionImpl) SetPongHandler(h func(string) error) {
	c.conn.SetPongHandler(h)
}
