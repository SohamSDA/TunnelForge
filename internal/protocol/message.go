// Package protocol defines the versioned messages exchanged by the relay and CLI.
package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const (
	Version         = 1
	MaxMessageBytes = 2 << 20
	MaxBodyBytes    = 1 << 20
)

type MessageType string

const (
	TypeRegister     MessageType = "register"
	TypeRegistered   MessageType = "registered"
	TypeHTTPRequest  MessageType = "http_request"
	TypeHTTPResponse MessageType = "http_response"
	TypeHeartbeat    MessageType = "heartbeat"
	TypeError        MessageType = "error"
)

var (
	ErrInvalidMessage     = errors.New("invalid protocol message")
	ErrUnsupportedVersion = errors.New("unsupported protocol version")
)

type Message struct {
	Version   int             `json:"version"`
	Type      MessageType     `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

type RegisterRequest struct {
	Token              string `json:"token,omitempty"`
	RequestedSubdomain string `json:"requested_subdomain,omitempty"`
}

type Registered struct {
	TunnelID string `json:"tunnel_id"`
	URL      string `json:"url"`
}

type HTTPRequest struct {
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Query   string              `json:"query,omitempty"`
	Headers map[string][]string `json:"headers,omitempty"`
	Body    []byte              `json:"body,omitempty"`
}

type HTTPResponse struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Body       []byte              `json:"body,omitempty"`
}

type Heartbeat struct {
	UnixMilliseconds int64 `json:"unix_milliseconds"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewMessage(messageType MessageType, requestID string, payload any) (Message, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Message{}, fmt.Errorf("encode payload: %w", err)
	}
	message := Message{Version: Version, Type: messageType, RequestID: requestID, Payload: encoded}
	if err := message.Validate(); err != nil {
		return Message{}, err
	}
	return message, nil
}

func Decode(reader io.Reader) (Message, error) {
	decoder := json.NewDecoder(io.LimitReader(reader, MaxMessageBytes+1))
	decoder.DisallowUnknownFields()

	var message Message
	if err := decoder.Decode(&message); err != nil {
		return Message{}, fmt.Errorf("%w: decode: %v", ErrInvalidMessage, err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return Message{}, fmt.Errorf("%w: multiple JSON values", ErrInvalidMessage)
	}
	if err := message.Validate(); err != nil {
		return Message{}, err
	}
	return message, nil
}

func (message Message) Validate() error {
	if message.Version != Version {
		return fmt.Errorf("%w: got %d, want %d", ErrUnsupportedVersion, message.Version, Version)
	}
	if !validType(message.Type) {
		return fmt.Errorf("%w: unknown type %q", ErrInvalidMessage, message.Type)
	}
	if len(message.Payload) == 0 || bytes.Equal(message.Payload, []byte("null")) {
		return fmt.Errorf("%w: payload is required", ErrInvalidMessage)
	}
	requestScoped := message.Type == TypeHTTPRequest || message.Type == TypeHTTPResponse
	if requestScoped && message.RequestID == "" {
		return fmt.Errorf("%w: request_id is required for %s", ErrInvalidMessage, message.Type)
	}
	if !requestScoped && message.RequestID != "" {
		return fmt.Errorf("%w: request_id is not allowed for %s", ErrInvalidMessage, message.Type)
	}
	return message.validatePayload()
}

func (message Message) PayloadAs(destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(message.Payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("%w: invalid %s payload: %v", ErrInvalidMessage, message.Type, err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("%w: multiple payload values", ErrInvalidMessage)
	}
	return nil
}

func (message Message) validatePayload() error {
	switch message.Type {
	case TypeRegister:
		var payload RegisterRequest
		return message.PayloadAs(&payload)
	case TypeRegistered:
		var payload Registered
		if err := message.PayloadAs(&payload); err != nil {
			return err
		}
		if payload.TunnelID == "" || payload.URL == "" {
			return fmt.Errorf("%w: tunnel_id and url are required", ErrInvalidMessage)
		}
	case TypeHTTPRequest:
		var payload HTTPRequest
		if err := message.PayloadAs(&payload); err != nil {
			return err
		}
		if payload.Method == "" || payload.Path == "" || payload.Path[0] != '/' {
			return fmt.Errorf("%w: HTTP method and absolute path are required", ErrInvalidMessage)
		}
		if len(payload.Body) > MaxBodyBytes {
			return fmt.Errorf("%w: request body exceeds %d bytes", ErrInvalidMessage, MaxBodyBytes)
		}
	case TypeHTTPResponse:
		var payload HTTPResponse
		if err := message.PayloadAs(&payload); err != nil {
			return err
		}
		if payload.StatusCode < 100 || payload.StatusCode > 599 {
			return fmt.Errorf("%w: invalid HTTP status code %d", ErrInvalidMessage, payload.StatusCode)
		}
		if len(payload.Body) > MaxBodyBytes {
			return fmt.Errorf("%w: response body exceeds %d bytes", ErrInvalidMessage, MaxBodyBytes)
		}
	case TypeHeartbeat:
		var payload Heartbeat
		return message.PayloadAs(&payload)
	case TypeError:
		var payload Error
		if err := message.PayloadAs(&payload); err != nil {
			return err
		}
		if payload.Code == "" || payload.Message == "" {
			return fmt.Errorf("%w: error code and message are required", ErrInvalidMessage)
		}
	}
	return nil
}

func validType(messageType MessageType) bool {
	switch messageType {
	case TypeRegister, TypeRegistered, TypeHTTPRequest, TypeHTTPResponse, TypeHeartbeat, TypeError:
		return true
	default:
		return false
	}
}

// Headers copies HTTP headers into a representation safe to pass through JSON.
func Headers(headers http.Header) map[string][]string {
	copy := make(map[string][]string, len(headers))
	for key, values := range headers {
		copy[key] = append([]string(nil), values...)
	}
	return copy
}
