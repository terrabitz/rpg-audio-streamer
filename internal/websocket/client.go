package websocket

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 40 * time.Second
)

type Client struct {
	ID    string
	hub   *Hub
	conn  *websocket.Conn
	send  chan []byte
	Token *auth.Token
}

func NewClient(hub *Hub, conn *websocket.Conn, token *auth.Token) *Client {
	return &Client{
		ID:    uuid.New().String(),
		hub:   hub,
		conn:  conn,
		send:  make(chan []byte, 256),
		Token: token,
	}
}

func (c *Client) Send(msg Message) error {
	j, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("couldn't marshal input: %w", err)
	}

	c.send <- j

	return nil
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(in string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		c.hub.route(message, c)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				fmt.Println("failed to write to socket")
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
