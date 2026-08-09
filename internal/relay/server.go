package relay

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/SohamSDA/TunnelForge/internal/protocol"
	"github.com/gorilla/websocket"
)

const maxClientQueue = 128

type messageSender interface {
	Send(context.Context, protocol.Message) error
	Close()
}

type Server struct {
	registry    *Registry
	pending     *PendingRequests
	publicURL   *url.URL
	token       string
	logger      *log.Logger
	pathRouting bool

	mu      sync.RWMutex
	clients map[string]messageSender
	upgrade websocket.Upgrader
}

type Option func(*Server)

func WithPathRouting() Option {
	return func(server *Server) { server.pathRouting = true }
}

func NewServer(publicURL, token string, logger *log.Logger, options ...Option) (*Server, error) {
	parsedURL, err := url.Parse(publicURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid public URL %q", publicURL)
	}
	pending, err := NewPendingRequests(1000, 30*time.Second)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = log.Default()
	}
	server := &Server{
		registry:  NewRegistry(),
		pending:   pending,
		publicURL: parsedURL,
		token:     token,
		logger:    logger,
		clients:   make(map[string]messageSender),
		upgrade:   websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	}
	for _, option := range options {
		option(server)
	}
	return server, nil
}

func (server *Server) TunnelHandler(w http.ResponseWriter, request *http.Request) {
	connection, err := server.upgrade.Upgrade(w, request, nil)
	if err != nil {
		return
	}
	clientID, err := secureID(16)
	if err != nil {
		_ = connection.Close()
		return
	}
	client := newWebSocketClient(connection)
	defer client.Close()

	message, err := client.read()
	if err != nil || message.Type != protocol.TypeRegister {
		server.sendProtocolError(client, "INVALID_REGISTRATION", "first message must register the tunnel")
		return
	}
	var registration protocol.RegisterRequest
	if err := message.PayloadAs(&registration); err != nil {
		server.sendProtocolError(client, "INVALID_REGISTRATION", err.Error())
		return
	}
	if server.token != "" && registration.Token != server.token {
		server.sendProtocolError(client, "UNAUTHORIZED", "invalid tunnel token")
		return
	}

	tunnel, err := server.registry.Register(clientID)
	if err != nil {
		server.sendProtocolError(client, "REGISTRATION_FAILED", err.Error())
		return
	}
	server.addClient(clientID, client)
	defer server.disconnect(clientID)

	publicURL := *server.publicURL
	if server.pathRouting {
		publicURL.Path = strings.TrimSuffix(publicURL.Path, "/") + "/t/" + tunnel.Subdomain
	} else {
		publicURL.Host = tunnel.Subdomain + "." + server.publicURL.Host
	}
	registered, err := protocol.NewMessage(protocol.TypeRegistered, "", protocol.Registered{TunnelID: tunnel.ID, URL: publicURL.String()})
	if err != nil || client.Send(request.Context(), registered) != nil {
		return
	}
	server.logger.Printf("tunnel registered subdomain=%s client=%s", tunnel.Subdomain, clientID)

	for {
		message, err := client.read()
		if err != nil {
			return
		}
		switch message.Type {
		case protocol.TypeHTTPResponse:
			var response protocol.HTTPResponse
			if message.PayloadAs(&response) == nil {
				_ = server.pending.Resolve(message.RequestID, response)
			}
		case protocol.TypeHeartbeat:
			// Receiving the message is enough to prove application-level liveness.
		}
	}
}

func (server *Server) PublicHandler(w http.ResponseWriter, request *http.Request) {
	subdomain, forwardedPath := server.route(request)
	if subdomain == "" {
		http.Error(w, "Tunnel subdomain required", http.StatusBadRequest)
		return
	}
	tunnel, exists := server.registry.Get(subdomain)
	if !exists {
		http.Error(w, "Tunnel not found", http.StatusNotFound)
		return
	}
	client, exists := server.client(tunnel.ClientID)
	if !exists {
		http.Error(w, "Tunnel unavailable", http.StatusBadGateway)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, request.Body, protocol.MaxBodyBytes))
	if err != nil {
		http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	requestID, result, err := server.pending.Register(request.Context(), tunnel.ID)
	if err != nil {
		http.Error(w, "Relay overloaded", http.StatusServiceUnavailable)
		return
	}
	forwarded := protocol.HTTPRequest{
		Method: request.Method, Path: forwardedPath, Query: request.URL.RawQuery,
		Headers: protocol.Headers(request.Header), Body: body,
	}
	removeHopByHop(forwarded.Headers)
	message, err := protocol.NewMessage(protocol.TypeHTTPRequest, requestID, forwarded)
	if err != nil {
		_ = server.pending.Cancel(requestID, err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if err := client.Send(request.Context(), message); err != nil {
		_ = server.pending.Cancel(requestID, err)
		http.Error(w, "Tunnel unavailable", http.StatusBadGateway)
		return
	}

	outcome := <-result
	if outcome.Err != nil {
		if errors.Is(outcome.Err, context.DeadlineExceeded) {
			http.Error(w, "Tunnel timed out", http.StatusGatewayTimeout)
		} else if request.Context().Err() == nil {
			http.Error(w, "Tunnel unavailable", http.StatusBadGateway)
		}
		return
	}
	removeHopByHop(outcome.Response.Headers)
	for key, values := range outcome.Response.Headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(outcome.Response.StatusCode)
	_, _ = w.Write(outcome.Response.Body)
}

func (server *Server) Close() {
	server.mu.Lock()
	clients := server.clients
	server.clients = make(map[string]messageSender)
	server.mu.Unlock()
	for clientID, client := range clients {
		client.Close()
		server.disconnect(clientID)
	}
}

func (server *Server) route(request *http.Request) (string, string) {
	if value := request.Header.Get("X-Tunnel-Subdomain"); value != "" {
		return strings.ToLower(value), request.URL.Path
	}
	if server.pathRouting {
		parts := strings.SplitN(strings.TrimPrefix(request.URL.Path, "/"), "/", 3)
		if len(parts) >= 2 && parts[0] == "t" && parts[1] != "" {
			path := "/"
			if len(parts) == 3 {
				path += parts[2]
			}
			return strings.ToLower(parts[1]), path
		}
		return "", ""
	}
	host := request.Host
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	baseHost := server.publicURL.Hostname()
	suffix := "." + strings.ToLower(baseHost)
	host = strings.ToLower(host)
	if strings.HasSuffix(host, suffix) {
		return strings.TrimSuffix(host, suffix), request.URL.Path
	}
	return "", ""
}

func (server *Server) addClient(id string, client messageSender) {
	server.mu.Lock()
	defer server.mu.Unlock()
	server.clients[id] = client
}

func (server *Server) client(id string) (messageSender, bool) {
	server.mu.RLock()
	defer server.mu.RUnlock()
	client, exists := server.clients[id]
	return client, exists
}

func (server *Server) disconnect(clientID string) {
	server.mu.Lock()
	delete(server.clients, clientID)
	server.mu.Unlock()
	for _, tunnel := range server.registry.RemoveClient(clientID) {
		server.pending.CancelTunnel(tunnel.ID, ErrTunnelDisconnected)
	}
}

func (server *Server) sendProtocolError(client messageSender, code, text string) {
	message, err := protocol.NewMessage(protocol.TypeError, "", protocol.Error{Code: code, Message: text})
	if err == nil {
		_ = client.Send(context.Background(), message)
	}
}

func removeHopByHop(headers map[string][]string) {
	for _, key := range []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade"} {
		delete(headers, key)
	}
}
