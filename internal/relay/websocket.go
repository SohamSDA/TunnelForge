package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/SohamSDA/TunnelForge/internal/protocol"
	"github.com/gorilla/websocket"
)

type websocketClient struct {
	connection *websocket.Conn
	queue      chan protocol.Message
	done       chan struct{}
	closeOnce  sync.Once
}

func newWebSocketClient(connection *websocket.Conn) *websocketClient {
	client := &websocketClient{connection: connection, queue: make(chan protocol.Message, maxClientQueue), done: make(chan struct{})}
	connection.SetReadLimit(protocol.MaxMessageBytes)
	_ = connection.SetReadDeadline(time.Now().Add(60 * time.Second))
	connection.SetPongHandler(func(string) error { return connection.SetReadDeadline(time.Now().Add(60 * time.Second)) })
	go client.writePump()
	return client
}

func (client *websocketClient) Send(ctx context.Context, message protocol.Message) error {
	select {
	case client.queue <- message:
		return nil
	case <-client.done:
		return errors.New("client disconnected")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (client *websocketClient) read() (protocol.Message, error) {
	_, encoded, err := client.connection.ReadMessage()
	if err != nil {
		return protocol.Message{}, err
	}
	return protocol.Decode(bytes.NewReader(encoded))
}

func (client *websocketClient) Close() {
	client.closeOnce.Do(func() {
		close(client.done)
		_ = client.connection.Close()
	})
}

func (client *websocketClient) writePump() {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case message := <-client.queue:
			_ = client.connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
			encoded, err := json.Marshal(message)
			if err != nil || client.connection.WriteMessage(websocket.TextMessage, encoded) != nil {
				client.Close()
				return
			}
		case <-ticker.C:
			_ = client.connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if client.connection.WriteMessage(websocket.PingMessage, nil) != nil {
				client.Close()
				return
			}
		case <-client.done:
			return
		}
	}
}
