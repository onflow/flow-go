package websockets

import (
	"time"

	"github.com/gorilla/websocket"
)

type WebsocketConnection interface {
	ReadJSON(v interface{}) error
	WriteJSON(v interface{}) error
	WriteControl(messageType int, deadline time.Time) error
	Close() error
	SetReadDeadline(deadline time.Time) error
	SetWriteDeadline(deadline time.Time) error
	SetPongHandler(h func(string) error)
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

func (c *WebsocketConnectionImpl) WriteControl(messageType int, deadline time.Time) error {
	return c.conn.WriteControl(messageType, nil, deadline)
}

func (c *WebsocketConnectionImpl) Close() error {
	return c.conn.Close()
}

func (c *WebsocketConnectionImpl) SetReadDeadline(deadline time.Time) error {
	return c.conn.SetReadDeadline(deadline)
}

func (c *WebsocketConnectionImpl) SetWriteDeadline(deadline time.Time) error {
	return c.conn.SetWriteDeadline(deadline)
}

func (c *WebsocketConnectionImpl) SetPongHandler(h func(string) error) {
	c.conn.SetPongHandler(h)
}
