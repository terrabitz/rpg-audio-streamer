package websocket

import (
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

type Client struct {
	hub   *Hub
	conn  *websocket.Conn
	send  chan []byte
	Token *auth.Token
}

func NewClient(hub *Hub, conn *websocket.Conn, token *auth.Token) *Client {
	return &Client{
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

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		c.hub.route(message, c)
	}
}

func (c *Client) WritePump() {
	defer c.conn.Close()

	for {
		message, ok := <-c.send
		if !ok {
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			fmt.Println("failed to write to socket")
			return
		}
	}
}
