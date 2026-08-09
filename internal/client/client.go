package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/SohamSDA/TunnelForge/internal/protocol"
	"github.com/gorilla/websocket"
)

type Config struct {
	LocalHost string
	LocalPort uint16
	RelayURL  *url.URL
	Token     string
	Logger    *log.Logger
	OnReady   func(protocol.Registered)
}

var ErrRejected = errors.New("relay rejected connection")

func Run(ctx context.Context, configuration Config) error {
	if configuration.Logger == nil {
		configuration.Logger = log.Default()
	}
	delay := time.Second
	for {
		err := runOnce(ctx, configuration)
		if ctx.Err() != nil {
			return nil
		}
		if errors.Is(err, ErrRejected) {
			return err
		}
		configuration.Logger.Printf("tunnel disconnected: %v; reconnecting in %s", err, delay)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
		if delay < 30*time.Second {
			delay *= 2
		}
	}
}

func runOnce(ctx context.Context, configuration Config) error {
	connection, _, err := websocket.DefaultDialer.DialContext(ctx, configuration.RelayURL.String(), nil)
	if err != nil {
		return err
	}
	defer connection.Close()
	connectionDone := make(chan struct{})
	defer close(connectionDone)
	go func() {
		select {
		case <-ctx.Done():
			_ = connection.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "client shutting down"), time.Now().Add(time.Second))
			_ = connection.Close()
		case <-connectionDone:
		}
	}()
	connection.SetReadLimit(protocol.MaxMessageBytes)

	writer := &safeWriter{connection: connection}
	registration, err := protocol.NewMessage(protocol.TypeRegister, "", protocol.RegisterRequest{Token: configuration.Token})
	if err != nil {
		return err
	}
	if err := writer.write(registration); err != nil {
		return err
	}

	_, encoded, err := connection.ReadMessage()
	if err != nil {
		return err
	}
	message, err := protocol.Decode(bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	if message.Type == protocol.TypeError {
		var response protocol.Error
		_ = message.PayloadAs(&response)
		return fmt.Errorf("%w: %s", ErrRejected, response.Message)
	}
	if message.Type != protocol.TypeRegistered {
		return errors.New("relay did not confirm tunnel registration")
	}
	var registered protocol.Registered
	if err := message.PayloadAs(&registered); err != nil {
		return err
	}
	if configuration.OnReady != nil {
		configuration.OnReady(registered)
	}

	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)
	go heartbeat(ctx, writer, heartbeatDone)

	localAddress := net.JoinHostPort(configuration.LocalHost, strconv.Itoa(int(configuration.LocalPort)))
	localClient := &http.Client{Timeout: 30 * time.Second}
	semaphore := make(chan struct{}, 64)
	for {
		_, encoded, err := connection.ReadMessage()
		if err != nil {
			return err
		}
		message, err := protocol.Decode(bytes.NewReader(encoded))
		if err != nil || message.Type != protocol.TypeHTTPRequest {
			continue
		}
		select {
		case semaphore <- struct{}{}:
		case <-ctx.Done():
			return nil
		}
		go func() {
			defer func() { <-semaphore }()
			forward(ctx, localClient, localAddress, writer, message)
		}()
	}
}

type safeWriter struct {
	mu         sync.Mutex
	connection *websocket.Conn
}

func (writer *safeWriter) write(message protocol.Message) error {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	_ = writer.connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return writer.connection.WriteJSON(message)
}

func heartbeat(ctx context.Context, writer *safeWriter, done <-chan struct{}) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			message, err := protocol.NewMessage(protocol.TypeHeartbeat, "", protocol.Heartbeat{UnixMilliseconds: time.Now().UnixMilli()})
			if err == nil {
				_ = writer.write(message)
			}
		case <-ctx.Done():
			return
		case <-done:
			return
		}
	}
}

func forward(ctx context.Context, localClient *http.Client, localAddress string, writer *safeWriter, message protocol.Message) {
	var incoming protocol.HTTPRequest
	if err := message.PayloadAs(&incoming); err != nil {
		writeForwardError(writer, message.RequestID, http.StatusBadGateway, err)
		return
	}
	localURL := url.URL{Scheme: "http", Host: localAddress, Path: incoming.Path, RawQuery: incoming.Query}
	request, err := http.NewRequestWithContext(ctx, incoming.Method, localURL.String(), bytes.NewReader(incoming.Body))
	if err != nil {
		writeForwardError(writer, message.RequestID, http.StatusBadGateway, err)
		return
	}
	request.Header = cloneHeaders(incoming.Headers)
	removeHopByHop(request.Header)
	response, err := localClient.Do(request)
	if err != nil {
		writeForwardError(writer, message.RequestID, http.StatusBadGateway, fmt.Errorf("local application unavailable: %w", err))
		return
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, protocol.MaxBodyBytes+1))
	if err != nil || len(body) > protocol.MaxBodyBytes {
		writeForwardError(writer, message.RequestID, http.StatusBadGateway, errors.New("local response body too large"))
		return
	}
	headers := protocol.Headers(response.Header)
	removeHopByHop(headers)
	forwarded, err := protocol.NewMessage(protocol.TypeHTTPResponse, message.RequestID, protocol.HTTPResponse{StatusCode: response.StatusCode, Headers: headers, Body: body})
	if err == nil {
		_ = writer.write(forwarded)
	}
}

func writeForwardError(writer *safeWriter, requestID string, status int, cause error) {
	message, err := protocol.NewMessage(protocol.TypeHTTPResponse, requestID, protocol.HTTPResponse{
		StatusCode: status, Headers: map[string][]string{"Content-Type": {"text/plain; charset=utf-8"}}, Body: []byte(cause.Error()),
	})
	if err == nil {
		_ = writer.write(message)
	}
}

func cloneHeaders(headers map[string][]string) http.Header {
	copy := make(http.Header, len(headers))
	for key, values := range headers {
		copy[key] = append([]string(nil), values...)
	}
	return copy
}

func removeHopByHop(headers map[string][]string) {
	for _, key := range []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade"} {
		delete(headers, key)
	}
}
